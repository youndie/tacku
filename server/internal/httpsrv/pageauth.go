package httpsrv

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/youndie/tacku/server/internal/auth"
)

// AuthConfigPath is where the page asks where to sign in.
//
// Not a well-known address, because it is not a standard document: the standard one — RFC 9728's
// protected resource metadata — describes a resource to a client that already knows its own
// identity, and this answers the other question, which is what identity the page should claim.
const AuthConfigPath = "/auth/config"

// PageAuth is how a person signs in to the browser client.
//
// Through the identity provider, by the same redirect an agent already uses — which is possible in a
// page and awkward in a window, and is why this arrived with the page rather than before it. What it
// replaces is a door of our own: a form exchanged a password for a session because OAuth 2.1 has no
// grant that does, and passwords therefore lived in this server's database.
//
// The audience is the page's own and not the MCP resource. A token good at both surfaces is exactly
// the confused deputy §12 is built against: an agent's token, spent by something that talked it out
// of an agent, would otherwise open the screens as well.
type PageAuth struct {
	// Issuer is the provider, and the same one the agent surface trusts. Two providers would be two
	// sets of people.
	Issuer string

	// JWKSURL is where that provider publishes its keys.
	JWKSURL string

	// Audience is what a token must carry to open the human surface: this deployment's page.
	Audience string

	// ClientID is what the page calls itself when it asks for a token. Public — a page keeps no
	// secret — so the flow is authorization code with PKCE and nothing else.
	ClientID string
}

func (p PageAuth) configured() bool {
	return p.Issuer != "" && p.JWKSURL != "" && p.Audience != "" && p.ClientID != ""
}

// authConfig tells the page where to send somebody and what to call itself.
//
// Public, and it has to be: it is read before anybody has signed in. It carries no secret because
// the page has none — the three values are the issuer, this deployment's audience and a client
// identifier, all of which appear in the address bar during the redirect anyway.
//
// It exists so that the bundle carries no configuration. The same file works on a laptop, on a stand
// and in production, and none of them needs a variable compiled in correctly.
func authConfig(page PageAuth) http.HandlerFunc {
	body, _ := json.Marshal(map[string]string{
		"issuer":   page.Issuer,
		"clientId": page.ClientID,
		"audience": page.Audience,
	})

	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

// requireHuman is the guard in front of every screen.
//
// Two doors, and the asymmetry is the point. The provider's token is the product's; the session this
// server issues itself is the instrument's, and it is compiled in or not — see the two files that
// provide `sessionDoor`. A door that is present but switched off by configuration is a door somebody
// switches on.
func requireHuman(page func(string) (auth.Principal, error), session func(string) (auth.Principal, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "Bearer "
			header := r.Header.Get("Authorization")
			if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
				challenge(w)
				return
			}
			token := header[len(prefix):]

			// Why it was refused, in the words of whichever door came closest to opening. A `401`
			// that says only "no" is the reason a wrong audience and an expired token and a
			// provider nobody can reach all look the same from a browser — and each has a different
			// fix. Nothing here is a secret: whoever is being told already holds the token and can
			// read every claim in it.
			var refusal error

			if page != nil {
				principal, err := page(token)
				if err == nil {
					next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
					return
				}
				refusal = err
			}
			if session != nil {
				principal, err := session(token)
				if err == nil {
					next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
					return
				}
				if refusal == nil {
					refusal = err
				}
			}

			refuseWithReason(w, refusal)
		})
	}
}

// pageVerifier turns a provider's token into a principal, or refuses it.
func pageVerifier(ctx context.Context, page PageAuth) (func(string) (auth.Principal, error), error) {
	if !page.configured() {
		return nil, nil
	}

	verify, err := auth.NewVerifier(auth.VerifierConfig{
		Issuer:   page.Issuer,
		Resource: page.Audience,
		JWKSURL:  page.JWKSURL,
	})
	if err != nil {
		return nil, err
	}

	return func(token string) (auth.Principal, error) {
		info, err := verify(ctx, token, nil)
		if err != nil {
			return auth.Principal{}, err
		}
		return auth.FromToken(info)
	}, nil
}
