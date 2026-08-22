package httpsrv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/render"
)

// kindUpdates is the endpoint kind of a live channel.
const kindUpdates = "updates_stream"

// updatesPath is where a person's own channel lives. One address, and what travels down it depends
// on who is holding it — see the note about topics below.
const updatesPath = "/updates"

// updates serves the journal as a push instead of a pull.
//
// **The channel is the journal, and that is the whole design.** There is no publish/subscribe, no
// hook on the write path and nothing to keep in step with it: this handler walks `Changes` from the
// reader's own cursor exactly as the catch-up screen does, and sends what it finds. A mutation
// performed by an agent over MCP, by a person over a form, or by anything added next month arrives
// here without that code knowing this file exists — because it went into the journal, which was
// always the point of having one.
//
// The price is latency bounded by the poll interval rather than zero, and it is a price worth
// naming: a second mechanism running beside the journal would be a second thing to be wrong, and
// the failure of a missed notification is silence, which is the hardest failure to notice.
//
// **The topic is the member, and it is derived from the token rather than taken from the request.**
// §14 leaves the transport to the implementation and says nothing about who may subscribe to what;
// a channel that accepted a topic name from its caller would hand anybody anybody else's updates
// for the asking. Nothing in the conformance kit can check that (it needs two live identities and
// the kit's transport is request/response), so it is checked here instead.
func updates(store domain.Store, seen domain.Seen, interval time.Duration) http.HandlerFunc {
	if interval <= 0 {
		interval = time.Second
	}

	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			fail(w, fmt.Errorf("%w: this server cannot stream", domain.ErrInvalidTask))
			return
		}

		// From where this person had read up to. The channel starts where the catch-up screen would
		// have started, so opening it is not a way to miss what the screen would have shown.
		visit, err := seen.Visit(r.Context(), principal.Provenance.OnBehalfOf)
		if err != nil {
			fail(w, err)
			return
		}
		from := visit.Pending

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			cursor, sent, err := emit(r, w, store, from)
			if err != nil {
				return
			}
			from = cursor
			if sent > 0 {
				flusher.Flush()
			}

			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
			}
		}
	}
}

// emit sends one frame per change and answers where the reader now stands.
//
// A frame carries the card of the task that changed, addressed by the identifier the board gave it.
// That identifier is why the board's tree is built from stable names and not from a counter over
// the walk: a component nobody can name is a component nobody can replace.
func emit(r *http.Request, w http.ResponseWriter, store domain.Store, from domain.Cursor) (domain.Cursor, int, error) {
	changes, next, err := store.Changes(r.Context(), from, pageSize)
	if err != nil {
		return from, 0, err
	}
	if len(changes) == 0 {
		return next, 0, nil
	}

	lastBy, err := store.LastActors(r.Context())
	if err != nil {
		return from, 0, err
	}

	sent := 0
	for _, change := range changes {
		task, err := store.Task(r.Context(), change.Task)
		if err != nil {
			// A change about something no longer readable is skipped rather than fatal: the reader
			// is watching a board, not auditing the journal.
			continue
		}
		frame := render.CardUpdate(task, lastBy[task.ID], moveURL)
		body, err := json.Marshal(frame)
		if err != nil {
			return from, sent, err
		}
		if _, err := fmt.Fprintf(w, "event: update_component\ndata: %s\n\n", body); err != nil {
			return from, sent, err
		}
		sent++
	}
	return next, sent, nil
}
