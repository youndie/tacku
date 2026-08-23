// Command tacku runs the tracker.
//
// One binary, one process: the agent surface and (later) the human one share a domain, a store and
// an idempotency mechanism, and splitting them into two programs would mean keeping three of those
// in step by hand.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/docsboard"
	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/httpsrv"
	"github.com/youndie/tacku/server/internal/mcpsrv"
	"github.com/youndie/tacku/server/internal/store/sqlite"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tacku:", err)
		os.Exit(1)
	}
}

func usage() error {
	return fmt.Errorf("usage: tacku mcp [-db path]\n" +
		"       tacku serve [-db path] [-addr :8080]\n\n" +
		"       tacku openapi [-resource URL]\n" +
		"       tacku add-member -id ID -email ADDRESS -name NAME [-db path]\n" +
		"       tacku measure [-db path] [-least N]\n" +
		"       tacku seed [-db path]\n\n" +
		"  mcp       serve the Model Context Protocol on stdin and stdout\n" +
		"  serve     serve it over HTTP as an OAuth 2.1 resource server\n" +
		"  openapi   print the description of the HTTP layer\n" +
		"  add-member  create one person who can sign in, printing their password once\n" +
		"  measure   print the two numbers the backlog is waiting on, or say why it will not\n" +
		"  seed      fill a fresh database with a demo board, for local work\n\n" +
		"stdio takes identity from the environment, which is what MCP asks of it:\n" +
		"  TACKU_AGENT_ID        the agent's own member identifier\n" +
		"  TACKU_AGENT_VERSION   the build acting, recorded on every change\n" +
		"  TACKU_ON_BEHALF_OF    the person the agent acts for\n\n" +
		"HTTP takes identity from the token, and needs the authorization server named:\n" +
		"  TACKU_RESOURCE        this server's canonical URI, and the audience a token must carry\n" +
		"  TACKU_ISSUER          the authorization server's issuer identifier\n" +
		"  TACKU_JWKS_URL        where that issuer publishes its signing keys\n" +
		"  TACKU_PAGE_AUDIENCE   what a token must carry to open a screen: this deployment's page\n" +
		"  TACKU_PAGE_CLIENT_ID  what the page calls itself when it asks the issuer for a token\n\n" +
		"The KOMPOT client carries tokens this server issues through the sign-in form:\n" +
		"  TACKU_SESSION_KEY     at least 32 characters; generated per run when unset\n" +
		"  TACKU_WIZARD_TTL      how long an untouched multi-step scenario is kept (default 30m)")
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "mcp":
		return runMCP(args[1:])
	case "serve":
		return runServe(args[1:])
	case "measure":
		return runMeasure(args[1:])
	case "openapi":
		return runOpenAPI(args[1:])
	case "add-member":
		return runAddMember(args[1:])
	case "seed":
		return runSeed(args[1:])
	default:
		return usage()
	}
}

func runMCP(args []string) error {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	path := flags.String("db", "tacku.db", "path to the database file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	version := os.Getenv("TACKU_AGENT_VERSION")
	fallback := domain.Agent(
		domain.MemberID(os.Getenv("TACKU_AGENT_ID")),
		version,
		domain.MemberID(os.Getenv("TACKU_ON_BEHALF_OF")),
	)
	deps := mcpsrv.Deps{Version: version, Fallback: &fallback}

	store, err := sqlite.Open(*path)
	if err != nil {
		return err
	}
	defer store.Close()
	deps.Store = store
	deps.Attempts = store

	// stdin and stdout carry the protocol, so anything printed there would corrupt it. Notice of
	// shutdown goes to stderr.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return mcpsrv.RunStdio(ctx, deps)
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	path := flags.String("db", "tacku.db", "path to the database file")
	addr := flags.String("addr", ":8080", "address to listen on")
	// The built browser client. Empty serves no page, which is what this server did for its whole
	// life and is still a legitimate shape: agents need no page.
	web := flags.String("web", "", "directory of the built browser client to serve")
	if err := flags.Parse(args); err != nil {
		return err
	}

	store, err := sqlite.Open(*path)
	if err != nil {
		return err
	}
	defer store.Close()

	// No fallback actor here, deliberately: over HTTP the actor comes from the token, and a
	// configured one would take over silently the moment that stopped working.
	key, err := sessionKey()
	if err != nil {
		return err
	}

	ttl, err := wizardTTL()
	if err != nil {
		return err
	}

	docs, err := docsSource()
	if err != nil {
		return err
	}

	handler, err := httpsrv.New(httpsrv.Config{
		Deps:       mcpsrv.Deps{Store: store, Attempts: store, Version: version()},
		Members:    store,
		Seen:       store,
		SessionKey: key,
		WizardTTL:  ttl,
		PageDir:    *web,
		DocsBoard:  docs,
		// The same issuer as the agent surface — two would be two sets of people — and an audience
		// of its own, so that a token minted for the agents cannot open a screen.
		Page: httpsrv.PageAuth{
			Issuer:   os.Getenv("TACKU_ISSUER"),
			JWKSURL:  os.Getenv("TACKU_JWKS_URL"),
			Audience: os.Getenv("TACKU_PAGE_AUDIENCE"),
			ClientID: os.Getenv("TACKU_PAGE_CLIENT_ID"),
		},
		Verifier: auth.VerifierConfig{
			Issuer:   os.Getenv("TACKU_ISSUER"),
			Resource: os.Getenv("TACKU_RESOURCE"),
			JWKSURL:  os.Getenv("TACKU_JWKS_URL"),
		},
	})
	if err != nil {
		return err
	}

	server := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	fmt.Fprintf(os.Stderr, "tacku: listening on %s as %s\n", *addr, os.Getenv("TACKU_RESOURCE"))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// sessionKey signs the pair the KOMPOT client carries.
//
// Generated when unset, and the consequence is stated rather than hidden: every restart invalidates
// every session, because the previous key is gone. Fine for a local run, wrong for anything else,
// and a deployment that skips the variable finds out at the first restart rather than at the first
// incident.
func sessionKey() ([]byte, error) {
	if configured := os.Getenv("TACKU_SESSION_KEY"); configured != "" {
		if len(configured) < 32 {
			return nil, fmt.Errorf("TACKU_SESSION_KEY must be at least 32 characters")
		}
		return []byte(configured), nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "tacku: TACKU_SESSION_KEY is unset; sessions will not survive a restart")
	return key, nil
}

// wizardTTL is how long an untouched multi-step scenario is kept.
//
// A variable rather than a constant because there is nothing else that can end a scenario: the
// protocol has no cancel transition, so a walk somebody abandoned is removed by a clock or not at
// all. The default is a choice and not a measurement — see wizard.DefaultTTL for the bounds it sits
// between — and whoever measures it must be able to change it without a build.
//
// An unreadable value stops the process instead of falling back to the default. A server silently
// running on thirty minutes while its configuration says two hours is the kind of wrong nobody
// finds, because everything works.
func wizardTTL() (time.Duration, error) {
	configured := os.Getenv("TACKU_WIZARD_TTL")
	if configured == "" {
		return 0, nil
	}
	ttl, err := time.ParseDuration(configured)
	if err != nil {
		return 0, fmt.Errorf("TACKU_WIZARD_TTL is not a duration: %w", err)
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("TACKU_WIZARD_TTL must be positive, got %s", configured)
	}
	return ttl, nil
}

// docsSource is the backlog the read-only view looks at, or nothing where none is configured.
//
// Every part of it comes from the environment and none of it has a default that names anything. This
// repository is public and stands beside closed ones; a repository name committed here cannot be
// taken back out of the clones, and scripts/no_private_names.py keeps fingerprints of the names that
// must not appear so that forgetting is caught by the gate rather than by a reader.
//
// A repository named without a credential is not corrected: a private source answers 404 to an
// anonymous request, which arrives as "there is no such repository" and sends whoever configured it
// looking for a typo.
func docsSource() (*docsboard.Source, error) {
	repo := os.Getenv("TACKU_DOCS_REPO")
	if repo == "" {
		return nil, nil
	}
	if !strings.Contains(repo, "/") {
		return nil, fmt.Errorf("TACKU_DOCS_REPO must be owner/name, got %q", repo)
	}

	ttl := time.Duration(0)
	if configured := os.Getenv("TACKU_DOCS_TTL"); configured != "" {
		parsed, err := time.ParseDuration(configured)
		if err != nil {
			return nil, fmt.Errorf("TACKU_DOCS_TTL is not a duration: %w", err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("TACKU_DOCS_TTL must be positive, got %s", configured)
		}
		ttl = parsed
	}

	return docsboard.New(docsboard.Config{
		Repo: repo,
		// Where the forge is. Set by the script that records the screen bodies, which points it at a
		// stub so that photographing this board needs neither a real repository nor a network — see
		// scripts/docs_stub.py. Empty everywhere else, which is the address of GitHub.
		API:   os.Getenv("TACKU_DOCS_API"),
		Ref:   os.Getenv("TACKU_DOCS_REF"),
		Root:  os.Getenv("TACKU_DOCS_ROOT"),
		Index: os.Getenv("TACKU_DOCS_INDEX"),
		Token: os.Getenv("TACKU_DOCS_TOKEN"),
		TTL:   ttl,
	}), nil
}

func version() string {
	if v := os.Getenv("TACKU_AGENT_VERSION"); v != "" {
		return v
	}
	return "0.1.0"
}

// runOpenAPI prints the description of the HTTP layer.
//
// Generated from the code that mounts the routes rather than written by hand, and committed so the
// conformance harness — which is Kotlin and cannot run this — reads a file. A committed artefact
// drifts from its generator, so a test compares them.
func runOpenAPI(args []string) error {
	flags := flag.NewFlagSet("openapi", flag.ContinueOnError)
	resource := flags.String("resource", "http://localhost:8080", "the server this description is of")
	if err := flags.Parse(args); err != nil {
		return err
	}
	_, err := os.Stdout.Write(httpsrv.OpenAPI(*resource))
	return err
}

// runSeed fills a fresh database so that a local run has something on screen.
//
// Also what a conformance walk needs: several of its checks reach their interesting paths only once
// an operation can succeed. Against an empty workspace the idempotency check saw a create fail for
// want of a board, and a failed attempt is deliberately not recorded — so the conflict it was
// looking for could never happen, and it reported a 404 where it wanted a 409.
func runSeed(args []string) error {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	path := flags.String("db", "tacku.db", "path to the database file")
	// A fixed instant, so that seeding twice produces the same database and the screen corpus taken
	// from it is the same bytes. Empty means now, which is what a person poking at a stand wants.
	at := flags.String("at", "", "stamp every entry at this RFC 3339 instant instead of now")
	if err := flags.Parse(args); err != nil {
		return err
	}

	store, err := sqlite.Open(*path)
	if err != nil {
		return err
	}
	if *at != "" {
		moment, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			return fmt.Errorf("-at is not an RFC 3339 instant: %w", err)
		}
		store = store.At(moment)
	}
	defer store.Close()

	ctx := context.Background()

	// The password is fixed and printed, because this command exists to fill a throwaway database
	// and a secret nobody can look up is not a secret, only an obstacle. It is never generated for
	// a real deployment: that is what inviting a person will be.
	for _, person := range []struct{ id, email, name string }{
		{"anna", "anna@tacku.team", "Anna Petrova"},
		{"ivan", "ivan@tacku.team", "Ivan Sokolov"},
		{"maria", "maria@tacku.team", "Maria Kim"},
	} {
		if _, err := store.AddMember(ctx, domain.MemberID(person.id), person.email, person.name, SeedPassword); err != nil {
			return err
		}
	}

	board, err := store.CreateBoard(ctx, "Sprint 24")
	if err != nil {
		return err
	}

	anna := domain.Human("anna")
	agent := domain.Agent("anna-agent", "dev", "anna")

	seeds := []struct {
		title    string
		status   domain.Status
		assignee domain.MemberID
		due      string
		by       domain.Provenance
	}{
		{"Fix login redirect loop", domain.StatusInReview, "anna", "2026-08-28", agent},
		{"Add rate limit to the auth endpoint", domain.StatusTodo, "ivan", "2026-08-29", anna},
		{"Audit the session cookie flags", domain.StatusTodo, "", "", anna},
		{"Ship the settings redesign", domain.StatusInProgress, "maria", "2026-08-28", anna},
		{"Update the onboarding copy", domain.StatusDone, "maria", "", agent},
		{"Write the migration note for the auth change", domain.StatusTodo, "anna", "", agent},
		{"Replace the deploy script", domain.StatusInProgress, "ivan", "", anna},
		{"Tighten the empty states copy", domain.StatusInReview, "maria", "", anna},
		{"Check the staging redirect trace", domain.StatusTodo, "anna", "2026-09-01", agent},
		{"Rotate the SSO signing keys", domain.StatusTodo, "anna", "", anna},
		{"Drop the unused feature flag", domain.StatusBlocked, "ivan", "", anna},
		{"Measure where people change status", domain.StatusTodo, "anna", "", anna},
	}

	for _, seed := range seeds {
		task, err := store.CreateTask(ctx, domain.Task{
			Board: board.ID, Title: seed.title, Status: seed.status,
			Assignee: seed.assignee, Due: seed.due,
		}, seed.by)
		if err != nil {
			return err
		}
		if seed.status == domain.StatusInReview {
			if _, err := store.Comment(ctx, task.ID, "Reproduced on staging.", agent); err != nil {
				return err
			}
		}
	}

	fmt.Fprintf(os.Stderr, "tacku: seeded %s with %d tasks on %q; sign in as anna@tacku.team / %s\n",
		*path, len(seeds), board.Title, SeedPassword)
	return nil
}

// SeedPassword is what the seeded people sign in with. Exported so a conformance harness can use it
// without a second place to keep it in step.
const SeedPassword = "conformance-stand"
