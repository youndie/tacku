package httpsrv

import (
	"net/http"
	"time"

	"github.com/youndie/tacku/server/internal/docsboard"
	"github.com/youndie/tacku/server/internal/render"
)

// docsBoardPath is the render package's constant under a local name, so that the route and the
// handler cannot be renamed apart.
const (
	docsBoardPath = render.DocsBoardPath
	docsItemPath  = render.DocsItemPath
)

// docsBoard shows a backlog that lives in a repository, and changes nothing.
//
// There is no submit beside it and no store behind it. The repository is where such an item is
// argued about, reviewed and merged; this end of the wire is a window, and a window that could write
// would owe an answer to "which of the two is right" — see docs/backlog/B-53.
func docsBoard(source *docsboard.Source, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		snapshot, failure := source.Load(r.Context())

		// Nothing in hand is the one case that is not a board. Everything else — including a source
		// that has gone away since the last reading — is the previous snapshot with a line above it
		// saying so, because an empty board and a dead connection look identical and the difference
		// is what a person came for.
		if snapshot.Empty() {
			where := source.Config()
			respond(w, r, render.UnreadableDocsBoard(
				principal.Provenance.OnBehalfOf, failure, where.Repo, where.Ref, where.Root))
			return
		}

		screen := render.DocsBoard{
			Person:   principal.Provenance.OnBehalfOf,
			Snapshot: snapshot,
			Now:      now(),
			Failure:  failure,
		}
		respond(w, r, screen.Screen())
	}
}

// docsItem shows one item of that backlog, and changes nothing either.
//
// The identifier is checked against what was read rather than against a shape: the item screen can
// only ever show something the board showed, and a request for anything else is a stale link or an
// invented one. Answering 404 says which.
func docsItem(source *docsboard.Source) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		snapshot, _ := source.Load(r.Context())
		item, found := snapshot.Item(r.PathValue("item"))
		if !found {
			http.Error(w, `{"error":"no such item"}`, http.StatusNotFound)
			return
		}

		screen := render.DocsItem{Person: principal.Provenance.OnBehalfOf, Item: item}
		respond(w, r, screen.Screen())
	}
}
