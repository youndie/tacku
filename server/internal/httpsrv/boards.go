package httpsrv

import (
	"net/http"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/forms"
	"github.com/youndie/tacku/server/internal/render"
)

const newBoardFormID = "new-board"

// newBoardForm closes the gap the design named: there were boards in the product and no screen on
// which one comes into being, so a first run met an empty list with nothing to press.
//
// The columns are not configurable, and that is a decision rather than an omission: the vocabulary
// has no list-of-fields with an add button, and four text inputs standing in for column names would
// be a form for the sake of having one.
func newBoardForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}

		form := forms.New(newBoardFormID)

		name := form.TextInput("name", "Name", "Platform hardening",
			[]forms.Rule{forms.Required("Give the board a name.")})

		columns := render.ReadOnlyField("board-columns", "Columns",
			"To do · In progress · In review · Done · Blocked",
			"Every board starts with these five. Renaming them comes later.")

		screen := render.Column("screen-new-board", 24,
			[]render.Modifier{render.Padding(32), render.Background(render.ColorSurface)},
			render.Column("new-board-heading", 6, nil,
				render.Text("new-board-title", "New board", render.TextDisplay),
				render.Text("new-board-note",
					"Boards are created by people. Your agent can fill one with tasks, but it cannot make one.",
					render.TextBodyMuted),
			),
			name,
			columns,
			render.Row("new-board-actions", 0, nil,
				render.Button("new-board-submit", "Create board", render.SubmitForm(form.FormID()),
					render.PaddingXY(14, 24), render.Background(render.ColorAccent)),
				render.Spacer("new-board-actions-spacer"),
			),
		)

		respond(w, r, form.Build(screen))
	}
}

func submitNewBoard(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}

		var request submitRequest
		if err := decodeSubmit(r, &request); err != nil {
			http.Error(w, `{"error":"the body is not a form submission"}`, http.StatusBadRequest)
			return
		}

		if _, err := store.CreateBoard(r.Context(), request.text("name")); err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"type": "navigate", "deeplink": render.LinkBoard})
	}
}

// submitSeen moves the boundary the catch-up screen is measured from.
//
// A submit rather than a navigate with a query on the end, which is what it used to be: marking
// everything seen changes state, and dressing a state change as navigation also produced a deeplink
// the graph could never carry.
// It is also the only way the boundary can be made to skip something: the automatic half moves it
// only over what a visit was offered, and the button says "all" and means it.
func submitSeen(seen domain.Seen, store domain.Store, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		latest, err := store.Latest(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		// The same clock the arrival is measured against. Timing this from anywhere else would put
		// the next arrival on either side of the gap for a reason nothing in the request explains.
		visit := domain.Dismiss(now(), latest)
		if err := seen.RecordVisit(r.Context(), principal.Provenance.OnBehalfOf, visit); err != nil {
			fail(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"type": "navigate", "deeplink": render.LinkCatchUp})
	}
}

const seenURL = "/submit/seen"
