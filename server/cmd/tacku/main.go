// Command tacku runs the tracker.
//
// One binary, one process: the agent surface and (later) the human one share a domain, a store and
// an idempotency mechanism, and splitting them into two programs would mean keeping three of those
// in step by hand.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/youndie/tacku/server/internal/domain"
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
	return fmt.Errorf("usage: tacku mcp [-db path]\n\n" +
		"  mcp   serve the Model Context Protocol on stdin and stdout\n\n" +
		"Identity comes from the environment, which is what MCP asks of a stdio server:\n" +
		"  TACKU_AGENT_ID        the agent's own member identifier\n" +
		"  TACKU_AGENT_VERSION   the build acting, recorded on every change\n" +
		"  TACKU_ON_BEHALF_OF    the person the agent acts for")
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "mcp":
		return runMCP(args[1:])
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

	deps := mcpsrv.Deps{
		Agent:      domain.MemberID(os.Getenv("TACKU_AGENT_ID")),
		Version:    os.Getenv("TACKU_AGENT_VERSION"),
		OnBehalfOf: domain.MemberID(os.Getenv("TACKU_ON_BEHALF_OF")),
	}

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
