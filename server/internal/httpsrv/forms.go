package httpsrv

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/forms"
	"github.com/youndie/tacku/server/internal/render"
)

const (
	kindForm   = "form"
	kindSubmit = "submit"
)

const newTaskFormID = "new-task"

// newTaskForm serves the form that creates a task.
//
// The due date is this deployment's own field type, which the vocabulary does not have and now has
// somewhere to be declared (§2.4). It was a text input with a mask and a regular expression for as
// long as there was nowhere to declare one — recorded as a compromise rather than hidden, and B-12
// held the decision until the mechanism existed.
func newTaskForm(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}

		boards, err := store.Boards(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		boardOptions := make([]render.SelectOption, 0, len(boards))
		for _, board := range boards {
			boardOptions = append(boardOptions, render.SelectOption{ID: string(board.ID), Label: board.Title})
		}

		form := forms.New(newTaskFormID)

		title := form.TextInput("title", "Title", "What needs to be done?",
			[]forms.Rule{forms.Required("Give the task a title.")})

		// The description, which until now a person could not write at all: the store has held a
		// body since the first migration, agents fill it through MCP, and the human surface had no
		// component that shows more than one line of anything (B-29). Optional on purpose — an
		// older client draws a placeholder here and cannot fill it (Q-42), and a required field
		// nobody can fill is a form nobody can submit.
		description := form.MultilineInput("description", "Description",
			"What does done look like?", "", render.DefaultLines, nil)

		board := form.Select("board", "Board", "Choose a board", boardOptions,
			[]forms.Rule{forms.Required("Every task belongs to a board.")})

		status := form.Select("status", "Status", "", statusOptions(), nil)

		// The rule stays. A field type of our own decides what a person does, not what the server
		// trusts: the value still arrives as text, from a client that may be anything, and a check
		// dropped because the control looks safe is a check dropped for the wrong reason.
		due := form.DateInput("due", "Due date", "", "", "", "Leave it empty if there is no deadline.",
			[]forms.Rule{forms.Regex(`^\d{4}-\d{2}-\d{2}$`, "Enter the date as YYYY-MM-DD, for example 2026-08-29.")})

		agent := form.Checkbox("agent_may_update", "Let my agent keep this task up to date")

		screen := render.Column("form-new-task", 20,
			[]render.Modifier{render.FillWidth(), render.Padding(32), render.Background(render.ColorSurface)},
			render.Text("form-new-task-title", "New task", render.TextDisplay),
			title,
			description,
			board,
			status,
			due,
			agent,
			render.Text("form-agent-hint",
				"It may change the status and post comments on your behalf. Every action stays in the history.",
				render.TextMeta),
			render.Row("form-actions", 12, nil,
				render.BackAction(render.LinkBoard),
				render.PrimaryButton("form-submit", "Create task", render.SubmitForm(form.FormID()),
					render.PaddingXY(12, 24)),
				render.Spacer("form-actions-spacer"),
			),
		)

		respond(w, r, form.Build(screen))
	}
}

func statusOptions() []render.SelectOption {
	options := make([]render.SelectOption, 0, len(domain.Statuses))
	for _, status := range domain.Statuses {
		options = append(options, render.SelectOption{ID: string(status), Label: render.StatusName(string(status))})
	}
	return options
}

// submitNewTask answers a KompotAction, never a redirect and never an empty 200 (§16.4): the client
// runs the answer through the same chain as any other intent.
func submitNewTask(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		var request submitRequest
		if err := decodeSubmit(r, &request); err != nil {
			http.Error(w, `{"error":"the body is not a form submission"}`, http.StatusBadRequest)
			return
		}

		// No idempotency here, and that is the change B-11 made: a repeat never reaches this
		// handler, so nothing it does — including the journal entry inside the store — can happen
		// twice.
		task, err := store.CreateTask(r.Context(), domain.Task{
			Board:  domain.BoardID(request.chosen("board")),
			Title:  request.text("title"),
			Body:   request.text("description"),
			Status: domain.Status(request.chosen("status")),
			Due:    request.text("due"),
		}, principal.Provenance)
		if err != nil {
			fail(w, err)
			return
		}

		// To the task itself, which exists now. It briefly answered with the board instead, because
		// a deeplink the client cannot resolve is one it must ignore — a button doing nothing, in
		// silence.
		writeJSON(w, http.StatusOK, map[string]any{
			"type": "navigate", "deeplink": render.LinkTask + string(task.ID),
		})
	}
}

// submitRequest is the envelope both a submit and a patch travel in. On a submit the fieldId is not
// significant (§16.4) — it names the field that changed, and on a submit nothing did.
type submitRequest struct {
	FormID  string                     `json:"formId"`
	FieldID string                     `json:"fieldId"`
	Values  map[string]json.RawMessage `json:"values"`
}

// text reads a text_value. Any other shape reads as empty rather than as an error: the schema is
// what refuses a wrong type, and guessing here would answer a second time in a different voice.
func (s submitRequest) text(field string) string {
	raw, ok := s.Values[field]
	if !ok {
		return ""
	}
	var value struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &value); err != nil || value.Type != "text_value" {
		return ""
	}
	return strings.TrimSpace(value.Text)
}

// chosen reads what a selection sends, which is not what a text input sends.
//
// A `select_input` carries an `entity_value` — an id and a title — and this server read every value
// as a `text_value` and quietly got nothing. Every selection in the product was affected: the status
// on a task, the board of a new task, the filter of a list, the target of a bulk move. Nothing
// failed: an empty status means "no status was chosen", so the request was refused or ignored on
// grounds that looked reasonable and were not true.
//
// Found by driving the published client rather than by reading the schema — the renderer casts to
// EntityValue, and a TextValue put there by hand throws. Wire types are cheap to get wrong in the
// direction that stays silent, and only the other half of the wire says so.
func (s submitRequest) chosen(field string) string {
	raw, ok := s.Values[field]
	if !ok {
		return ""
	}
	var value struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(raw, &value); err != nil || value.Type != "entity_value" {
		return ""
	}
	return strings.TrimSpace(value.ID)
}

func unauthenticated(w http.ResponseWriter) {
	http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// board serves the task board. No input on it, so it is a screen and it caches.
func board(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}
		person := principal.Provenance.OnBehalfOf

		boards, err := store.Boards(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		// A workspace with no boards is a new workspace, not a missing one. Answering 404 here sent
		// a first-run visitor to an error page, and it was a conformance walk against an empty
		// database that found it: every unit test seeds a board first, so the branch was never
		// taken.
		if len(boards) == 0 {
			respond(w, r, render.EmptyWorkspace(person))
			return
		}

		tasks, err := store.Tasks(r.Context(), boards[0].ID)
		if err != nil {
			fail(w, err)
			return
		}

		// Asked of the journal rather than derived from a column on the task: duplicated state is
		// the kind that disagrees later. It used to be a walk over the first 500 entries, which is
		// correct exactly until a team writes the 501st.
		lastBy, err := store.LastActors(r.Context())
		if err != nil {
			fail(w, err)
			return
		}

		respond(w, r, render.Board{
			Title:   boards[0].Title,
			MoveURL: moveURL,
			Person:  person,
			Tasks:   tasks,
			LastBy:  lastBy,
		}.Screen())
	}
}

const moveURL = "/submit/move"

// submitMove is what a card's button performs.
//
// A submit endpoint like any other: it answers a KompotAction and it requires an idempotency key.
// An agent retries; so does a person with a slow connection and an impatient finger.
func submitMove(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		var request submitRequest
		if err := decodeSubmit(r, &request); err != nil {
			http.Error(w, `{"error":"the body is not a form submission"}`, http.StatusBadRequest)
			return
		}

		// The surface is named here and nowhere else: nothing in the request says which screen it
		// came from (Q-32), so the address is the only witness — and this address is the board's
		// alone.
		if _, err := store.MoveTask(r.Context(),
			domain.TaskID(request.text("task")),
			domain.Status(request.text("status")),
			principal.Provenance,
			domain.SurfaceBoard); err != nil {
			fail(w, err)
			return
		}

		// A navigate back to the board, which the client runs through the same chain as any other
		// intent — and which puts the moved card in its new column without this endpoint having to
		// describe a tree.
		writeJSON(w, http.StatusOK, map[string]any{"type": "navigate", "deeplink": "app://board"})
	}
}

// decodeSubmit reads the envelope both a submit and a patch travel in.
func decodeSubmit(r *http.Request, into *submitRequest) error {
	return json.NewDecoder(r.Body).Decode(into)
}
