package httpsrv

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/render"
)

// screenKind and the other kinds are what the OpenAPI description declares per route, and what a
// conformance run reads to know which serialiser a response should satisfy. A client picks by kind,
// never by path.
const (
	kindScreen = "screen"
	kindPage   = "page"
	kindGraph  = "graph"
)

// catchUp serves the one screen of this product that takes no input.
//
// Asking for it is what the server treats as a person arriving, which is an assumption and is
// recorded as one (Q-30): the protocol has no arrival signal, and §16.2 designs for the client
// re-requesting a screen it already holds. Under a client that revalidates on a timer the visit
// would never end and the feed would only grow — the harmless direction of being wrong, which is
// why the assumption is allowed to stand.
func catchUp(store domain.Store, seen domain.Seen, gap time.Duration, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}
		person := principal.Provenance.OnBehalfOf

		latest, err := store.Latest(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		visit, err := seen.Visit(r.Context(), person)
		if err != nil {
			fail(w, err)
			return
		}

		// The boundary moves here and nowhere else on a read, and it moves to where the *previous*
		// visit ended rather than to the end of the journal. Advancing to the end would empty the
		// screen being rendered, which is the one thing a catch-up screen must never do.
		arrived := visit.Arrive(now(), latest, gap)
		if err := seen.RecordVisit(r.Context(), person, arrived); err != nil {
			fail(w, err)
			return
		}

		// From where this person had read up to, which is what makes the screen a catch-up rather
		// than a list of everything that ever happened.
		from := arrived.Boundary

		changes, next, err := store.Changes(r.Context(), from, pageSize)
		if err != nil {
			fail(w, err)
			return
		}
		// Counted rather than taken from the page and the workspace, which is what the headline
		// claims to be about.
		total, boards, err := store.CountSince(r.Context(), from)
		if err != nil {
			fail(w, err)
			return
		}

		more := ""
		if len(changes) == pageSize {
			more = "/pages/changes?cursor=" + string(next)
		}

		respond(w, r, render.Feed{
			Person:   person,
			SeenURL:  seenURL,
			Changes:  changes,
			Total:    total,
			Boards:   boards,
			Away:     arrived.Away,
			NextPage: more,
		}.Screen())
	}
}

// changesPage is the same list, continued. One address serves both the next page and a reload of
// the first, and only the client knows which it asked for.
func changesPage(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}

		changes, next, err := store.Changes(r.Context(), cursorOf(r), pageSize)
		if err != nil {
			fail(w, err)
			return
		}

		items := make([]render.Component, 0, len(changes))
		for _, change := range changes {
			items = append(items, render.ChangeRow(change))
		}

		// A walk has to end, and it ends by this being empty. Handing back a next address for a
		// page that came up short would loop a client for ever over nothing.
		more := ""
		if len(changes) == pageSize {
			more = "/pages/changes?cursor=" + string(next)
		}

		respond(w, r, render.Page(items, more))
	}
}

const pageSize = 20

func cursorOf(r *http.Request) domain.Cursor {
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		return domain.Cursor(raw)
	}
	return domain.Start
}

// respond writes the body and its ETag, and answers 304 when the client already holds it.
//
// The tag is the hash of the bytes actually sent, which is the only definition that cannot drift
// from them. It works at all only because the tree is built from stable identifiers: were they
// generated per request, every body would differ, the tag would change every time and a 304 would
// never happen — the failure mode being a cache that is present, correct and useless.
func respond(w http.ResponseWriter, r *http.Request, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		fail(w, err)
		return
	}

	sum := sha256.Sum256(encoded)
	tag := `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
	w.Header().Set("ETag", tag)
	w.Header().Set("Content-Type", "application/json")

	if match := r.Header.Get("If-None-Match"); match == tag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

func fail(w http.ResponseWriter, err error) {
	// The body of an error is a convention of this server rather than part of the protocol, and it
	// is the shape SPEC.md §16.8 describes.
	w.Header().Set("Content-Type", "application/json")
	switch {
	case errorIs(err, domain.ErrNotFound):
		w.WriteHeader(http.StatusNotFound)
	case errorIs(err, domain.ErrConflict):
		w.WriteHeader(http.StatusConflict)
	case errorIs(err, domain.ErrInvalidTask), errorIs(err, domain.ErrBadCursor):
		w.WriteHeader(http.StatusUnprocessableEntity)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
