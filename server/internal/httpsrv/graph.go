package httpsrv

import (
	"errors"
	"net/http"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/render"
)

// navigationGraph lists the screens a client can reach without new client code.
//
// Only the simple ones: a route here promises that fetching the endpoint yields a tree and nothing
// else is needed. A screen wanting idempotency, a subscription or a multi-step scenario stays out
// and ships with a release.
func navigationGraph() http.HandlerFunc {
	graph := struct {
		Routes []render.Route `json:"routes"`
	}{Routes: render.Graph}

	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}
		respond(w, r, graph)
	}
}

func principalOf(r *http.Request) (auth.Principal, error) {
	return auth.PrincipalFrom(r.Context())
}

func errorIs(err, target error) bool { return errors.Is(err, target) }
