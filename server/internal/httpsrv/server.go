// Package httpsrv serves the agent surface over HTTP as an OAuth 2.1 resource server.
//
// The authorization server is somebody else's: the MCP specification puts its implementation out of
// scope, and writing one here would be a second product inside this one. What is not optional is
// behaving correctly as the resource — publishing protected resource metadata, refusing tokens
// minted for anybody else, and answering a challenge a client can act on.
package httpsrv

import (
	"fmt"
	"net/http"
	"strings"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/idem"
	"github.com/youndie/tacku/server/internal/mcpsrv"
)

// MetadataPath is fixed by RFC 9728, which a compliant MCP server MUST implement.
const MetadataPath = "/.well-known/oauth-protected-resource"

// MCPPath is where the protocol is served. It is part of this server's canonical URI, so it is also
// part of what a token's audience has to match.
const MCPPath = "/mcp"

type Config struct {
	Deps     mcpsrv.Deps
	Verifier auth.VerifierConfig

	// Seen is where each person had read up to, which the catch-up screen is measured from.
	Seen domain.Seen

	// Members and SessionKey belong to the KOMPOT surface, whose tokens this server issues itself
	// through the sign-in form. See auth.Sessions for why there are two token systems here.
	Members    domain.Members
	SessionKey []byte
}

// New assembles the mux.
func New(config Config) (http.Handler, error) {
	if config.Deps.Fallback != nil {
		return nil, fmt.Errorf("httpsrv: a fallback actor must not be configured here — over HTTP the actor comes from the token")
	}

	verifier, err := auth.NewVerifier(config.Verifier)
	if err != nil {
		return nil, err
	}

	sessions, err := auth.NewSessions(config.SessionKey)
	if err != nil {
		return nil, err
	}
	if config.Members == nil {
		return nil, fmt.Errorf("httpsrv: no member directory, so nobody could ever sign in")
	}

	readWrite, err := mcpsrv.New(config.Deps)
	if err != nil {
		return nil, err
	}
	readOnly, err := mcpsrv.NewReadOnly(config.Deps)
	if err != nil {
		return nil, err
	}

	// Which server a caller gets is decided by the token they presented. The specification allows
	// exactly this — the tool set "MAY vary by the authorization presented on the request" — and it
	// is the only way scopes can be honoured at all here: every tool call is a POST to one path, so
	// the transport cannot answer 403 for a call it has not yet parsed. Hiding a tool a caller may
	// not use is better than refusing it after the model has spent a turn on it.
	handler := mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		if info := sdkauth.TokenInfoFromContext(request.Context()); info != nil {
			for _, scope := range info.Scopes {
				if scope == auth.ScopeWrite {
					return readWrite
				}
			}
		}
		return readOnly
	}, nil)

	metadataURL := strings.TrimSuffix(config.Verifier.Resource, MCPPath) + MetadataPath

	guarded := sdkauth.RequireBearerToken(verifier, &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL,
		// Only the read scope is required to reach the endpoint. Demanding the write scope here
		// would deny a read-only agent the whole server, and the specification's own remedy for
		// finer granularity is a narrower tool list rather than a wider challenge.
		Scopes: []string{auth.ScopeRead},
	})(handler)

	metadata := sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               config.Verifier.Resource,
		AuthorizationServers:   []string{config.Verifier.Issuer},
		ScopesSupported:        []string{auth.ScopeRead, auth.ScopeWrite},
		BearerMethodsSupported: []string{"header"},
	})

	// The human surface, behind its own tokens.
	//
	// An earlier version guarded it with the OAuth check as well, on the argument that two token
	// systems are two places to get authorisation wrong. That simplification lasted until the
	// sign-in form had to exist: KOMPOT issues its pair through `update_session` (§12.4), and
	// OAuth 2.1 has no grant by which a form could exchange a password for a token. Each surface
	// now carries what its own specification asks for, and an MCP token is not accepted here —
	// spending a credential bound by audience to somewhere else is the confused deputy arriving
	// through the front door.
	screens := http.NewServeMux()
	screens.Handle("GET /screens/catch-up", catchUp(config.Deps.Store, config.Seen))
	screens.Handle("GET /pages/changes", changesPage(config.Deps.Store))
	screens.Handle("GET /screens/board", board(config.Deps.Store))
	screens.Handle("GET /forms/new-task", newTaskForm(config.Deps.Store))
	screens.Handle("POST /submit/new-task", submitNewTask(config.Deps.Store))
	screens.Handle("POST /submit/move", submitMove(config.Deps.Store))
	screens.Handle("GET /forms/my-tasks", myTasks(config.Deps.Store))
	screens.Handle("GET /pages/tasks", tasksPage(config.Deps.Store))
	screens.Handle("GET /forms/new-board", newBoardForm())
	screens.Handle("POST /submit/new-board", submitNewBoard(config.Deps.Store))
	screens.Handle("POST /submit/seen", submitSeen(config.Seen, config.Deps.Store))
	screens.Handle("GET /graph", navigationGraph())

	mux := http.NewServeMux()
	mux.Handle(MetadataPath, metadata)
	mux.Handle(MCPPath, guarded)
	// Idempotency wraps the handlers rather than living inside them: a repeat then never reaches
	// one, so nothing a handler does can happen twice — not the operation and not the journal
	// entry, the log line or the update frame beside it.
	//
	// And it sits *inside* the session check, not outside. The other order answered an anonymous
	// caller with 400 for a missing idempotency key: telling somebody who has not authenticated
	// what else their request lacks, and hiding the reason it was going to be refused anyway.
	guardedScreens := requireSession(sessions, idem.Middleware(config.Deps.Attempts, screens))

	mux.Handle("/screens/", guardedScreens)
	mux.Handle("/pages/", guardedScreens)
	mux.Handle("/forms/", guardedScreens)
	mux.Handle("/submit/", guardedScreens)
	mux.Handle("/graph", guardedScreens)

	// Public, and the only route of this surface that is: a person with no session has to be able
	// to reach the form that starts one.
	mux.Handle("GET /forms/sign-in", loginForm())
	mux.Handle("POST "+LoginPath, submitLogin(config.Members, sessions))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux, nil
}
