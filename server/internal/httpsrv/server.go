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

	protect := sdkauth.RequireBearerToken(verifier, &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL,
		Scopes:              []string{auth.ScopeRead},
	})

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

	// The human surface. Behind the same bearer check as the agent one: two token systems on one
	// server would be two places to get authorisation wrong, and SPEC.md §16.7 asks only that the
	// client send a bearer token — not where it came from.
	screens := http.NewServeMux()
	screens.Handle("GET /screens/catch-up", catchUp(config.Deps.Store))
	screens.Handle("GET /pages/changes", changesPage(config.Deps.Store))
	screens.Handle("GET /graph", navigationGraph())

	mux := http.NewServeMux()
	mux.Handle(MetadataPath, metadata)
	mux.Handle(MCPPath, guarded)
	mux.Handle("/screens/", protect(screens))
	mux.Handle("/pages/", protect(screens))
	mux.Handle("/graph", protect(screens))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux, nil
}
