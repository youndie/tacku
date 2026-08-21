package httpsrv

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"

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
func catchUp(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}

		changes, next, err := store.Changes(r.Context(), cursorOf(r), pageSize)
		if err != nil {
			fail(w, err)
			return
		}
		boards, err := store.Boards(r.Context())
		if err != nil {
			fail(w, err)
			return
		}

		more := ""
		if len(changes) == pageSize {
			more = "/pages/changes?cursor=" + string(next)
		}

		respond(w, r, render.Feed{
			Person:   principal.Provenance.OnBehalfOf,
			Changes:  changes,
			Boards:   len(boards),
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
