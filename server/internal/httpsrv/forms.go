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

const editTaskFormID = "edit-task"

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

		// A choice of one is not a choice, and the mockup does not draw one: on its create screen
		// the board is context in the header — the person came from it — rather than a field. Until
		// the button that opens this form carries which board it was pressed on, the honest middle
		// is to stop asking when there is nothing to ask.
		//
		// Not declared as a field either, and a guard here insisted on that before I had thought it
		// through: a schema entry no input shows is a value nobody can fill, so the read-only line
		// is what a person sees and the server is what decides. A second board declares the field
		// again, because then there is something to ask.
		var board render.Component
		if len(boards) == 1 {
			board = render.ReadOnlyField("field-board", "Board", boards[0].Title, "")
		} else {
			board = form.Select("board", "Board", "Choose a board", boardOptions,
				[]forms.Rule{forms.Required("Every task belongs to a board.")})
		}

		status := form.Select("status", "Status", "", statusOptions(), nil)

		// The rule stays. A field type of our own decides what a person does, not what the server
		// trusts: the value still arrives as text, from a client that may be anything, and a check
		// dropped because the control looks safe is a check dropped for the wrong reason.
		due := form.DateInput("due", "Due date", "", "", "", "Leave it empty if there is no deadline.",
			[]forms.Rule{forms.Regex(`^\d{4}-\d{2}-\d{2}$`, "Enter the date as YYYY-MM-DD, for example 2026-08-29.")})

		agent := form.Checkbox("agent_may_update", "Let my agent keep this task up to date")

		principal, _ := principalOf(r)
		screen := framedForm("form-new-task", principal.Provenance.OnBehalfOf, render.LinkBoard,
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

// framedForm puts a form where every other screen of this product lives: beside the rail.
//
// Two things it settles, and the second is why it is a list. The rail was simply missing from the
// two screens that create and change a task, so walking into one felt like leaving the product —
// and once the rail is there the frame is a row, which the toolkit draws as a single item that
// cannot scroll. There is no scrolling container in the vocabulary (Q-66); the one thing that
// scrolls its own content is a paginated list, so the form's own nodes are its items.
//
// A list with no next page and no empty state: it is a scroller, and saying so plainly here is
// better than a reader wondering which page never loads.
func framedForm(id string, person domain.MemberID, current string, body ...render.Component) render.Component {
	return render.Row(id+"-frame", 0,
		[]render.Modifier{render.FillWidth(), render.FillHeight(), render.Background(render.ColorSurface)},
		render.Navigation(person, current),
		render.Rule(id+"-nav-rule", render.RuleDp, render.ColorDivider, false),
		// The padding is **inside** the list, not on it. On it, it insets the scrolling viewport:
		// the content then slides under a frame and is cut by it at both ends, which is exactly what
		// it looked like — a form with a margin that ate its own top and bottom. Inside, the padding
		// scrolls with the content, which is what a person means by a margin.
		//
		// One item rather than one per node: a form is eight nodes, so there is nothing to gain from
		// laziness, and a single column keeps the spacing between fields the business of the column
		// that owns them rather than of the list.
		render.PaginatedList(id, []render.Component{
			render.Column(id+"-body", 20, []render.Modifier{render.FillWidth(), render.Padding(32)}, body...),
		}, "", nil, render.Weight(1)),
	)
}

// editTaskForm serves the form that changes a task, filled in with what it currently is.
//
// It exists because nothing here could change a title. The journal has had `title_edited` and
// `body_edited` as kinds since it was written and the store had no call that produced either: the
// vocabulary of what happened was ahead of what could happen, which is a shape of gap that no test
// notices — every entry the server writes is one it can write.
//
// Filled in through `initialValue` on the field definitions rather than through the inputs, which
// carry no value at all. Until that arrived upstream a form could only create.
//
// Three fields, not four. The assignee is missing on purpose: the store knows how to look one member
// up and not how to list them, so a select here would need a new way to read — and handing a task to
// somebody is arguably its own act with its own journal kind rather than a line on an edit form.
// Written down in B-46 rather than added quietly.
func editTaskForm(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}

		id := domain.TaskID(r.PathValue("task"))
		task, err := store.Task(r.Context(), id)
		if err != nil {
			fail(w, err)
			return
		}

		form := forms.New(editTaskFormID)

		title := form.TextInput("title", "Title", "What needs to be done?",
			[]forms.Rule{forms.Required("Give the task a title.")}, forms.Filled(task.Title))

		description := form.MultilineInput("description", "Description",
			"What does done look like?", task.Body, render.DefaultLines, nil)

		due := form.DateInput("due", "Due date", task.Due, "", "", "Leave it empty if there is no deadline.",
			[]forms.Rule{forms.Regex(`^\d{4}-\d{2}-\d{2}$`, "Enter the date as YYYY-MM-DD, for example 2026-08-29.")})

		principal, _ := principalOf(r)
		screen := framedForm("form-edit-task", principal.Provenance.OnBehalfOf, render.LinkBoard,
			render.Text("form-edit-task-title", "Edit task", render.TextDisplay),
			render.Text("form-edit-task-meta", string(task.ID), render.TextMeta),
			title,
			description,
			due,
			render.Row("edit-actions", 12, nil,
				render.BackTask(id),
				render.PrimaryButton("edit-submit", "Save changes", render.SubmitForm(form.FormID()),
					render.PaddingXY(12, 24)),
				render.Spacer("edit-actions-spacer"),
			),
		)

		respond(w, r, form.Build(screen))
	}
}

// submitEditTask applies what changed, and only what changed.
//
// Four calls rather than one, because the journal distinguishes four kinds and a history that says
// "edited" makes a reader open the task to find out which half moved. A field that arrives equal to
// what is stored writes nothing: the store's own edit refuses to record a change that changed
// nothing, so a person who opens the form and saves it untouched leaves no trace.
func submitEditTask(store domain.Store) http.HandlerFunc {
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

		id := domain.TaskID(r.PathValue("task"))
		by := principal.Provenance

		if title := request.text("title"); title != "" {
			if _, err := store.Retitle(r.Context(), id, title, by); err != nil {
				fail(w, err)
				return
			}
		}
		if _, err := store.Rewrite(r.Context(), id, request.text("description"), by); err != nil {
			fail(w, err)
			return
		}
		if _, err := store.SetDue(r.Context(), id, request.text("due"), by); err != nil {
			fail(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"type": "navigate", "deeplink": render.LinkTask + string(id)})
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
		// The board a person could not choose. When there is exactly one, the form shows it rather
		// than offering it, so a client that submits nothing for it is not a client in error — and
		// the alternative, trusting every client to echo a value it was never given a control for,
		// is a rule that holds until somebody writes a second client.
		chosenBoard := request.chosen("board")
		if chosenBoard == "" {
			if boards, err := store.Boards(r.Context()); err == nil && len(boards) == 1 {
				chosenBoard = string(boards[0].ID)
			}
		}

		task, err := store.CreateTask(r.Context(), domain.Task{
			Board:  domain.BoardID(chosenBoard),
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
