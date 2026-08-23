package httpsrv

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/forms"
	"github.com/youndie/tacku/server/internal/render"
	"github.com/youndie/tacku/server/internal/wizard"
)

// The two kinds of SPEC.md §16.1 that had no endpoint behind them until now: wizard_start answers a
// KompotFormResponse plus the header naming the scenario, wizard_resume answers a KompotAction.
const (
	kindWizardStart  = "wizard_start"
	kindWizardResume = "wizard_resume"
)

// WizardHeader carries the identifier of a scenario back to its start.
//
// It is now one direction only, and the other direction is no longer ours. §16.7 declared this
// header for the answer of `wizard_start`; the request side was described nowhere and the resume
// envelope had no field for it, so a header of our own naming carried it back (Q-50) — a private
// convention a client written against the specification could not have guessed.
//
// kompot 0.21 put it in the envelope: `WizardResumeRequest.wizardId`. A field and not a header,
// because the type belongs to the toolkit — the schema describes it and the conformance kit can
// check it, neither of which is true of a header. The header stays where the specification put it
// and no longer carries anything back.
const WizardHeader = "X-Tacku-Scenario"

const (
	wizardStartPath  = "/wizard/new-task"
	wizardResumePath = "/wizard/new-task/step"
)

// The flow is named by its own identifier rather than by the form of a step: a scenario outlives
// each of its steps, and the store checks that a resume belongs to the flow it was started for.
const newTaskFlowID = "new-task-flow"

const (
	wizardBasicsFormID  = "new-task-basics"
	wizardDetailsFormID = "new-task-details"
)

// wizardStep is one step of a flow: what it is called on the wire, which fields belong to it, and
// how to build it.
//
// fields is declared beside the builder rather than read out of it, and a test compares the two.
// The list is what decides which values a step is allowed to overwrite, so a step that grew a field
// and forgot to say so would silently keep a stale value from an earlier visit.
type wizardStep struct {
	id     string
	formID string
	fields []string
	build  func(context.Context, domain.Store, map[string]json.RawMessage) (forms.Response, error)
}

// newTaskFlow is the walk that creates a task in two steps.
//
// The product already creates a task through a plain form, and this flow does not replace it: what
// B-39 is about is the wire — the two endpoint kinds nothing in this server had ever answered — and
// choosing which of the two ways to create a task survives is a decision for the design rather than
// for the endpoint that proves the mechanism works.
func newTaskFlow() []wizardStep {
	return []wizardStep{
		{
			id: "basics", formID: wizardBasicsFormID,
			// `board` is conditional: with one board there is nothing to choose and the step does
			// not declare it. The check that holds the two creation surfaces together compares what
			// each actually declares, so both had to learn the same rule — which is the point of
			// that check, and it found this the moment only one of them had.
			fields: []string{"title", "description"},
			build:  wizardBasics,
		},
		{
			id: "details", formID: wizardDetailsFormID,
			fields: []string{"status", "due", "agent_may_update"},
			build:  wizardDetails,
		},
	}
}

// wizardBasics is what a task cannot be created without.
//
// The step is drawn empty on every visit, including a visit reached by Back, and that is not an
// omission: no field definition and no input of this profile carries an initial value, so a server
// holding what a person typed has no way of putting it back on the screen (Q-53).
func wizardBasics(ctx context.Context, store domain.Store, _ map[string]json.RawMessage) (forms.Response, error) {
	boards, err := store.Boards(ctx)
	if err != nil {
		return forms.Response{}, err
	}
	options := make([]render.SelectOption, 0, len(boards))
	for _, board := range boards {
		options = append(options, render.SelectOption{ID: string(board.ID), Label: board.Title})
	}

	form := forms.New(wizardBasicsFormID)
	title := form.TextInput("title", "Title", "What needs to be done?",
		[]forms.Rule{forms.Required("Give the task a title.")})

	// A choice of one is not a choice: the mockup draws the board as context on this screen rather
	// than as a field, and with a single board the server knows the answer. Shown, not asked.
	var board render.Component
	if len(boards) == 1 {
		board = render.ReadOnlyField("wizard-basics-board", "Board", boards[0].Title, "")
	} else {
		board = form.Select("board", "Board", "Choose a board", options,
			[]forms.Rule{forms.Required("Every task belongs to a board.")})
	}
	// The same field the one-screen form offers, and the reason it is here is not symmetry for its
	// own sake: two ways to create a task that take different facts make one of them the way that
	// loses something, and nobody would find out from a failure — the task simply arrives without a
	// description. A check below holds the two field sets together from now on.
	description := form.MultilineInput("description", "Description",
		"What does done look like?", "", render.DefaultLines, nil)

	return form.Build(render.Column("wizard-basics-fields", 20,
		[]render.Modifier{render.FillWidth(), render.FillHeight(), render.Padding(32), render.Background(render.ColorSurface)},
		title,
		board,
		description,
		render.Text("wizard-basics-next", "Status, deadline and what your agent may do come next.",
			render.TextMeta),
	)), nil
}

// wizardDetails is everything a task can be created without.
func wizardDetails(_ context.Context, _ domain.Store, _ map[string]json.RawMessage) (forms.Response, error) {
	form := forms.New(wizardDetailsFormID)
	status := form.Select("status", "Status", "", statusOptions(), nil)
	due := form.DateInput("due", "Due date", "", "", "",
		"Leave it empty if there is no deadline.",
		[]forms.Rule{forms.Regex(`^\d{4}-\d{2}-\d{2}$`, "Enter the date as YYYY-MM-DD, for example 2026-08-29.")})
	agent := form.Checkbox("agent_may_update", "Let my agent keep this task up to date")

	return form.Build(render.Column("wizard-details-fields", 20,
		[]render.Modifier{render.FillWidth(), render.FillHeight(), render.Padding(32), render.Background(render.ColorSurface)},
		status,
		due,
		agent,
		render.Text("wizard-details-agent-hint",
			"It may change the status and post comments on your behalf. Every action stays in the history.",
			render.TextMeta),
	)), nil
}

// wizardStart opens a scenario and answers its first step.
//
// A GET, and the choice is recorded rather than assumed: §16.1 gives every kind the shape of its
// answer and never a method (Q-50). What settles it here is that the answer is a document and the
// state behind it is invisible to the product — no task, no journal entry — while a second call is
// required to start from a blank sheet anyway.
//
// The answer carries no ETag, unlike the other document endpoints of this server. The interesting
// half of it is the header, and a 304 would hand a caller a fresh scenario beside a body it was told
// not to re-read.
func wizardStart(store domain.Store, scenarios *wizard.Store) http.HandlerFunc {
	flow := newTaskFlow()

	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}

		first := flow[0]
		built, err := first.build(r.Context(), store, nil)
		if err != nil {
			fail(w, err)
			return
		}

		id, err := scenarios.Start(principal.Provenance.OnBehalfOf, newTaskFlowID, first.id)
		if err != nil {
			fail(w, err)
			return
		}

		w.Header().Set(WizardHeader, string(id))
		writeJSON(w, http.StatusOK, forms.Response{
			Schema: built.Schema,
			Screen: wizardScreenOf(first, 0, len(flow), false, built.Screen),
		})
	}
}

// wizardResumeRequest is `WizardResumeRequest`: a transition and the values of the step just filled
// in. It names neither the scenario nor the step — the first travels in a header, the second the
// server is required to remember (§13.1).
type wizardResumeRequest struct {
	// Named by the protocol as of 0.21 and read from here rather than from a header of ours.
	WizardID   string `json:"wizardId"`
	Transition struct {
		Type   string `json:"type"`
		StepID string `json:"stepId"`
	} `json:"transition"`
	Values map[string]json.RawMessage `json:"values"`
}

// wizardResume answers a transition with a KompotAction: the next step while the flow continues, and
// whatever the flow ends in once it is over (§11.5).
func wizardResume(store domain.Store, scenarios *wizard.Store) http.HandlerFunc {
	flow := newTaskFlow()

	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := principalOf(r)
		if err != nil {
			unauthenticated(w)
			return
		}
		person := principal.Provenance.OnBehalfOf

		var request wizardResumeRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, `{"error":"the body is not a wizard transition"}`, http.StatusBadRequest)
			return
		}

		// From the envelope. A header is still accepted for one release so that a client built
		// against the previous shape is not broken by this server moving first — §15 orders the
		// rollout, and this is the same rule seen from the serving side.
		id := wizard.ID(request.WizardID)
		if id == "" {
			id = wizard.ID(r.Header.Get(WizardHeader))
		}
		state, err := scenarios.Resume(id, person)
		if err != nil {
			// The one answer to an expired scenario, one that never existed and one belonging to
			// somebody else. 404 rather than 410: 410 would assert that this identifier was real
			// once, and the three cases are deliberately not told apart (Q-25).
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}

		here := slices.IndexFunc(flow, func(step wizardStep) bool { return step.id == state.Step })
		if here < 0 {
			// The flow changed under a scenario that was already walking it. Answering as if the
			// scenario were gone is the truth from where the caller stands.
			writeJSON(w, http.StatusNotFound, map[string]string{"error": wizard.ErrGone.Error()})
			return
		}
		state.Values = wizardMerge(state.Values, flow[here].fields, request.Values)

		switch request.Transition.Type {
		case "next":
			if here+1 >= len(flow) {
				refuse(w, "there is no step after this one")
				return
			}
			state.History = append(state.History, flow[here].id)
			state.Step = flow[here+1].id

		case "back":
			if len(state.History) == 0 {
				refuse(w, "this is the first step, so there is nothing behind it")
				return
			}
			state.Step = state.History[len(state.History)-1]
			state.History = state.History[:len(state.History)-1]

		case "jump_to":
			if !slices.Contains(state.History, request.Transition.StepID) {
				// Only backwards, and only over ground already walked: jumping forward would skip
				// the steps whose values the finish is built from.
				refuse(w, "that step has not been reached yet")
				return
			}
			state.History = state.History[:slices.Index(state.History, request.Transition.StepID)]
			state.Step = request.Transition.StepID

		case "finish":
			if here != len(flow)-1 {
				refuse(w, "the flow is not on its last step")
				return
			}
			wizardFinish(w, r, store, scenarios, id, principal.Provenance, state)
			return

		default:
			// The hierarchy of transitions is closed and has no fallback (§2.2), so a fifth one is
			// a client that is not speaking this protocol rather than a newer client.
			refuse(w, "that is not a transition of this protocol")
			return
		}

		if err := scenarios.Save(id, person, state); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}

		next := slices.IndexFunc(flow, func(step wizardStep) bool { return step.id == state.Step })
		built, err := flow[next].build(r.Context(), store, state.Values)
		if err != nil {
			fail(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"type":   "wizard_step_result",
			"formId": flow[next].formID,
			"schema": built.Schema,
			"screen": wizardScreenOf(flow[next], next, len(flow), len(state.History) > 0, built.Screen),
		})
	}
}

// wizardFinish performs what the whole walk was for and drops the scenario.
//
// The state is dropped by the store on the one ending the server is told about, and that is also
// what keeps a repeated finish from creating a second task: the second one finds no scenario at all.
// The idempotency key protects the same thing one layer higher and for a different failure — a
// retry after an answer was lost — which is why both are here (Q-51).
func wizardFinish(
	w http.ResponseWriter, r *http.Request,
	store domain.Store, scenarios *wizard.Store,
	id wizard.ID, by domain.Provenance, state wizard.State,
) {
	values := submitRequest{Values: state.Values}

	// The board the walk did not ask for. With one board the step shows it instead of offering it,
	// so nothing arrives under that name and the server supplies what it already knew — the same
	// rule the one-screen form follows, in the same words, because two rules would be two answers.
	chosenBoard := values.chosen("board")
	if chosenBoard == "" {
		if boards, err := store.Boards(r.Context()); err == nil && len(boards) == 1 {
			chosenBoard = string(boards[0].ID)
		}
	}

	task, err := store.CreateTask(r.Context(), domain.Task{
		Board:  domain.BoardID(chosenBoard),
		Title:  values.text("title"),
		Body:   values.text("description"),
		Status: domain.Status(values.chosen("status")),
		Due:    values.text("due"),
	}, by)
	if err != nil {
		fail(w, err)
		return
	}

	if err := scenarios.Finish(id, by.OnBehalfOf); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"type": "navigate", "deeplink": render.LinkTask + string(task.ID),
	})
}

// wizardScreenOf wraps a step in the component that makes it a step.
//
// Nothing in the contract says the screen of a wizard_start or of a wizard_step_result has to be a
// wizard_screen (Q-52); a plain column would satisfy the schema and leave the client with no
// progress, no Back and no Finish to draw, because those are drawn from this node alone (§11.1).
func wizardScreenOf(step wizardStep, index, total int, canGoBack bool, content render.Component) render.Component {
	// §11.3: the identifier here and the identifier of the content's schema must agree — it is what
	// the client builds its own transitions from.
	return render.WizardScreen("wizard-"+step.id, step.formID, step.id,
		index, render.Steps(total), canGoBack, render.WizardFinishLabel(), content)
}

// wizardMerge writes a step's answers over what was held for that step.
//
// Only the fields of the step being left are touched, and a field of that step which did not arrive
// is cleared rather than kept: a value is omitted from a submission when it is empty (§9.4), so
// keeping the old one would make an emptied field impossible to empty.
func wizardMerge(held map[string]json.RawMessage, fields []string, arrived map[string]json.RawMessage) map[string]json.RawMessage {
	merged := map[string]json.RawMessage{}
	for field, value := range held {
		merged[field] = value
	}
	for _, field := range fields {
		if value, ok := arrived[field]; ok {
			merged[field] = value
			continue
		}
		delete(merged, field)
	}
	return merged
}
