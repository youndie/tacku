package httpsrv

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/forms"
	"github.com/youndie/tacku/server/internal/render"
)

// The selection mode: several tasks and one status, in one operation.
//
// Dragging a card is not in the vocabulary and the design withdrew the request for it, but the pain
// behind it was real — moving a handful of tasks meant a handful of requests. What replaces it is
// entirely inside the vocabulary: a checkbox per task, one selector, one submit.
//
// Three things about the shape are decided elsewhere and only carried out here:
//
//   - the route is `/submit/` plus the form identifier, because nothing on the wire says where a
//     form submits (Q-24);
//   - the list does not page, because an input arriving as a page declares no field (Q-25);
//   - the move is all-or-nothing, because a repeat under one key must reproduce one outcome (Q-26).
const (
	bulkFormPath   = "/forms/bulk-move"
	bulkSubmitPath = "/submit/" + render.BulkFormID
)

// bulkFieldPrefix keeps the checkboxes apart from the selector in one flat set of values.
//
// A submit carries `values` as one map of fieldId to value, so the field naming a task and the field
// naming the target status live in the same namespace. The prefix is what makes "which of these is a
// task" a question about the name rather than about a list the handler has to be told separately.
const bulkFieldPrefix = "task-"

// bulkLimit is how many tasks the screen offers at once.
//
// A limit exists because a form declares every field it draws in the envelope that carries it, and
// the continuation of a list arrives as a page, which carries no schema — so a checkbox on page two
// would name a field nobody declared (Q-25). The number is a choice and not a measurement: it is
// large enough for the "sort out a sprint's worth" the design named, and small enough that the
// envelope stays a screenful. What is not a choice is saying out loud how many are not shown.
const bulkLimit = 25

// bulkMoveForm serves the selection screen.
func bulkMoveForm(store domain.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := principalOf(r); err != nil {
			unauthenticated(w)
			return
		}

		tasks, err := allTasks(r, store)
		if err != nil {
			fail(w, err)
			return
		}
		lastBy, err := store.LastActors(r.Context())
		if err != nil {
			fail(w, err)
			return
		}

		hidden := 0
		if len(tasks) > bulkLimit {
			hidden = len(tasks) - bulkLimit
			tasks = tasks[:bulkLimit]
		}

		form := forms.New(render.BulkFormID)

		// The full selector, unlike the single-step button on a card: this is the screen where
		// moving backwards, into Blocked, or across two columns is possible at all.
		status := form.Select("status", "Move selected to", "Choose a status…", statusOptions(),
			[]forms.Rule{forms.Required("Choose where the selected tasks go.")})

		boxes := make(map[domain.TaskID]render.Component, len(tasks))
		for _, task := range tasks {
			boxes[task.ID] = form.Checkbox(bulkFieldPrefix+string(task.ID), task.Title)
		}

		respond(w, r, form.Build(render.BulkMove{
			Tasks:  tasks,
			LastBy: lastBy,
			Boxes:  boxes,
			Hidden: hidden,
		}.Screen(status)))
	}
}

// submitBulkMove applies one status to everything that was ticked.
//
// No idempotency handling of its own: the middleware above every submit sees the repeat before this
// handler does, so neither the moves nor the journal entries beside them can happen twice.
func submitBulkMove(store domain.Store) http.HandlerFunc {
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

		status := domain.Status(request.text("status"))
		selected := request.ticked(bulkFieldPrefix)

		// Refused on the merits rather than as a malformed request: the body is a perfectly good
		// submission of a form somebody has not finished filling in, and §16.8 keeps those apart so
		// that a client knows whether to fix the call or the data.
		if len(selected) == 0 {
			refuse(w, "Tick at least one task before moving them.")
			return
		}
		if status == "" {
			refuse(w, "Choose where the selected tasks go.")
			return
		}

		results, err := store.MoveTasks(r.Context(), selected, status, principal.Provenance, domain.SurfaceBulk)
		if err != nil {
			// A named task that is not there is refused on the merits, not answered 404. The
			// endpoint was found; it is one item of the payload that was not, and the person can
			// act on that — the screen was drawn before somebody else deleted the task.
			//
			// The refusal does not name which task, and that is a real loss rather than an
			// oversight: the store says so in a sentence written for a log, and passing it through
			// would put "domain: not found" in front of a person (§14). Naming it properly needs an
			// error that carries the identifier, which is worth doing when something reads it —
			// B-32 is where that becomes true.
			if errorIs(err, domain.ErrNotFound) {
				refuse(w, "One of the selected tasks is no longer there, so nothing moved. "+
					"Open the screen again and choose from what is left.")
				return
			}
			fail(w, err)
			return
		}

		writeJSON(w, http.StatusOK, bulkAnswer{
			Type:     "navigate",
			Deeplink: render.LinkBoard,
			Outcome:  outcomeOf(results),
		})
	}
}

// bulkAnswer is the action, plus what happened to each task.
//
// The action is what the client acts on (§16.4). The extra field is where the outcome goes, and it
// is an extension of the declared body — which §3 allows, the reading side being required to ignore
// what it does not know, and which this server already does for the field identifier of a validation
// error (Q-02).
//
// It is here rather than absent because the alternative is worse than useless: the ten actions of
// the profile carry no way to say anything, so an outcome that is not simply "everything moved" has
// nowhere at all to live (Q-26). This is the place B-32 will read from, and a repeat under the same
// key is answered with it again — which is the other half of what B-32 needs. The same JSON rather
// than the same bytes: the recorded outcome is stored as a raw message and marshalling one compacts
// it, so the replay loses the newline the first answer ended with. Nothing reads the bytes.
//
// Named after the operation rather than something generic, so that a field the protocol may add to
// `navigate` later cannot silently collide with ours.
type bulkAnswer struct {
	Type     string        `json:"type"`
	Deeplink string        `json:"deeplink"`
	Outcome  []taskOutcome `json:"bulkOutcome"`
}

type taskOutcome struct {
	Task    string `json:"task"`
	From    string `json:"from"`
	To      string `json:"to"`
	Outcome string `json:"outcome"`
}

func outcomeOf(results []domain.MoveResult) []taskOutcome {
	out := make([]taskOutcome, 0, len(results))
	for _, result := range results {
		out = append(out, taskOutcome{
			Task:    string(result.Task),
			From:    string(result.From),
			To:      string(result.To),
			Outcome: string(result.Outcome),
		})
	}
	return out
}

// ticked returns the tasks whose checkbox came back true.
//
// Ordered by number rather than by identifier or by map order. Map order is random in Go and would
// scatter the journal entries of one operation differently on every run; and sorting the identifiers
// as text puts TAC-10 before TAC-2, which is the mistake this project has already paid for once.
func (s submitRequest) ticked(prefix string) []domain.TaskID {
	var found []domain.TaskID
	for fieldID, raw := range s.Values {
		name, ok := strings.CutPrefix(fieldID, prefix)
		if !ok || !boolean(raw) {
			continue
		}
		found = append(found, domain.TaskID(name))
	}

	sort.Slice(found, func(i, j int) bool {
		left, leftErr := found[i].Number()
		right, rightErr := found[j].Number()
		if leftErr != nil || rightErr != nil {
			// Not a valid identifier, so there is no number to order by. Kept rather than dropped:
			// the store refuses it by name, and a value silently discarded here would turn a
			// mistyped identifier into a move that quietly did less than it was asked to.
			return string(found[i]) < string(found[j])
		}
		return left < right
	})
	return found
}

// boolean reads a boolean_value. Any other shape reads as false, for the reason submitRequest.text
// gives: the schema is what refuses a wrong type, and answering again here in a different voice
// would make two rules out of one.
//
// An untouched checkbox does not arrive at all — a field with no value is omitted rather than sent
// as null (§1.4) — so "absent" and "false" have to mean the same thing, and here they do.
func boolean(raw json.RawMessage) bool {
	var value struct {
		Type  string `json:"type"`
		Value bool   `json:"value"`
	}
	if err := json.Unmarshal(raw, &value); err != nil || value.Type != "boolean_value" {
		return false
	}
	return value.Value
}

// refuse is 422 with finished text: a request that is well formed and not acceptable (§16.8).
func refuse(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": message})
}
