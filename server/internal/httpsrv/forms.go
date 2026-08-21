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

const newTaskFormID = "task_create"

// newTaskForm serves the form that creates a task.
//
// The due date is a text field with a pattern, because the vocabulary has no date type. It is a
// compromise recorded rather than hidden: a person types a date instead of picking one, and B-12
// holds the decision about whether that is worth a field type of our own.
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

		board := form.Select("board", "Board", "Choose a board", boardOptions,
			[]forms.Rule{forms.Required("Every task belongs to a board.")})

		status := form.Select("status", "Status", "", statusOptions(), nil)

		// Required first, then the pattern. A regex passes an empty value so that an optional field
		// can stay blank, so the order is what decides which message a person is shown.
		due := form.TextInput("due", "Due date", "YYYY-MM-DD",
			[]forms.Rule{forms.Regex(`^\d{4}-\d{2}-\d{2}$`, "Enter the date as YYYY-MM-DD, for example 2026-08-29.")},
			forms.Mask("0000-00-00"))

		agent := form.Checkbox("agent_may_update", "Let my agent keep this task up to date")

		screen := render.Column("form-new-task", 20,
			[]render.Modifier{render.Padding(32), render.Background(render.ColorSurface)},
			render.Text("form-new-task-title", "New task", render.TextDisplay),
			title,
			board,
			status,
			render.Column("form-due-block", 4, nil,
				due,
				render.Text("form-due-hint", "YYYY-MM-DD", render.TextMeta),
			),
			agent,
			render.Text("form-agent-hint",
				"It may change the status and post comments on your behalf. Every action stays in the history.",
				render.TextMeta),
			render.Row("form-actions", 0, nil,
				render.Button("form-submit", "Create task", render.SubmitForm(form.FormID()),
					render.PaddingXY(14, 24), render.Background(render.ColorAccent)),
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
			Board:  domain.BoardID(request.text("board")),
			Title:  request.text("title"),
			Status: domain.Status(request.text("status")),
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
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}

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
			respond(w, r, render.EmptyWorkspace())
			return
		}

		tasks, err := store.Tasks(r.Context(), boards[0].ID)
		if err != nil {
			fail(w, err)
			return
		}

		// One pass over the journal rather than a query per card. The board is small and the
		// alternative is a column on the task, which would duplicate what the journal already
		// knows — and duplicated state is the kind that disagrees later.
		changes, _, err := store.Changes(r.Context(), domain.Start, 500)
		if err != nil {
			fail(w, err)
			return
		}
		lastBy := make(map[domain.TaskID]domain.Provenance, len(tasks))
		for _, change := range changes {
			lastBy[change.Task] = change.By
		}

		respond(w, r, render.Board{
			Title:   boards[0].Title,
			MoveURL: moveURL,
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

		if _, err := store.MoveTask(r.Context(),
			domain.TaskID(request.text("task")),
			domain.Status(request.text("status")),
			principal.Provenance); err != nil {
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
