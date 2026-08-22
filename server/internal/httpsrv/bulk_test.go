package httpsrv_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/render"
	"github.com/youndie/tacku/server/internal/spec"
)

const bulkForm = "/forms/bulk-move"

const bulkSubmit = "/submit/bulk-move"

// bulkBody is a submission of the selection form: one status and the boxes, ticked or not.
//
// The unticked ones are sent explicitly as false rather than left out. A real client omits a field
// it has no value for, so both shapes reach this endpoint, and only sending the ticked ones would
// leave the branch that has to read a false and ignore it untested.
func bulkBody(status string, boxes map[string]bool) string {
	names := make([]string, 0, len(boxes))
	for name := range boxes {
		names = append(names, name)
	}
	sort.Strings(names)

	values := make([]string, 0, len(boxes)+1)
	if status != "" {
		values = append(values, `"status":{"type":"text_value","text":"`+status+`"}`)
	}
	for _, name := range names {
		values = append(values,
			fmt.Sprintf(`"task-%s":{"type":"boolean_value","value":%t}`, name, boxes[name]))
	}
	return `{"formId":"bulk-move","fieldId":"","values":{` + strings.Join(values, ",") + `}}`
}

func (r *resource) statusOf(t *testing.T, id domain.TaskID) domain.Status {
	t.Helper()
	task, err := r.store.Task(context.Background(), domain.TaskID(id))
	if err != nil {
		t.Fatal(err)
	}
	return task.Status
}

func (r *resource) bodyOf(t *testing.T, response *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	return body
}

// The item this whole endpoint exists for: several tasks, one action, one request.
func TestOneSubmitMovesEveryTickedTaskAndLeavesTheRestWhereTheyAre(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	body := bulkBody("done", map[string]bool{"TAC-1": true, "TAC-2": true, "TAC-3": false})
	response := r.post(t, bulkSubmit, token, "bulk-1", body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the bulk move answered %d: %s", response.StatusCode, r.bodyOf(t, response))
	}

	for _, id := range []domain.TaskID{"TAC-1", "TAC-2"} {
		if got := r.statusOf(t, id); got != domain.StatusDone {
			t.Errorf("%s stands in %q after being ticked, want done", id, got)
		}
	}
	// The one that matters most: a checkbox that came back false must not travel with the ones that
	// came back true. Without this the endpoint could be "move everything on the screen" and every
	// other assertion here would still pass.
	if got := r.statusOf(t, "TAC-3"); got == domain.StatusDone {
		t.Error("an unticked task moved as well, so the selection decided nothing")
	}
}

// The other half of the requirement: provenance per task, not per operation.
func TestEveryTaskThatMovedGetsItsOwnJournalEntry(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	before := r.journalLength(t)

	response := r.post(t, bulkSubmit, token, "bulk-2",
		bulkBody("blocked", map[string]bool{"TAC-1": true, "TAC-2": true}))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the bulk move answered %d: %s", response.StatusCode, r.bodyOf(t, response))
	}

	if got := r.journalLength(t) - before; got != 2 {
		t.Fatalf("two tasks moved and the journal grew by %d entries; a bulk move must be readable "+
			"task by task, or the feed and the agent's cursor lose it", got)
	}

	changes, _, err := r.store.Changes(context.Background(), domain.Start, 500)
	if err != nil {
		t.Fatal(err)
	}
	moved := map[domain.TaskID]domain.Change{}
	for _, change := range changes[len(changes)-2:] {
		moved[change.Task] = change
	}
	for _, id := range []domain.TaskID{"TAC-1", "TAC-2"} {
		change, ok := moved[id]
		if !ok {
			t.Fatalf("no journal entry names %s", id)
		}
		if change.Kind != domain.ChangeStatusMoved || change.To != "blocked" {
			t.Errorf("the entry for %s is %q -> %q of kind %q", id, change.From, change.To, change.Kind)
		}
		if change.By.Executor.Member == "" {
			t.Errorf("the entry for %s names nobody", id)
		}
	}
}

// A repeat under one key does nothing a second time, and answers what the first attempt answered —
// down to the per-task outcome, which is where B-32 will read from.
func TestARepeatedBulkMoveLeavesNoSecondTraceAndReplaysItsOutcome(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	body := bulkBody("in_progress", map[string]bool{"TAC-1": true, "TAC-2": true})

	first := r.post(t, bulkSubmit, token, "bulk-3", body)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("the first attempt answered %d", first.StatusCode)
	}
	firstBody := r.bodyOf(t, first)
	afterFirst := r.journalLength(t)

	second := r.post(t, bulkSubmit, token, "bulk-3", body)
	if second.StatusCode != http.StatusOK {
		t.Fatalf("the repeat answered %d", second.StatusCode)
	}
	secondBody := r.bodyOf(t, second)

	if got := r.journalLength(t); got != afterFirst {
		t.Errorf("the repeat wrote %d more journal entries", got-afterFirst)
	}
	// Compared with the surrounding space removed, and the reason is worth writing down because it
	// looks like sloppiness and is not. The recorded outcome is held as a json.RawMessage, and
	// marshalling one compacts it — so the replay is the same JSON, never the same bytes. Nothing
	// downstream reads the bytes (a submit carries no ETag), and a test demanding them equal would
	// be testing the storage format of the idempotency record rather than the guarantee.
	if strings.TrimSpace(string(firstBody)) != strings.TrimSpace(string(secondBody)) {
		t.Errorf("the repeat answered %s where the first answered %s", secondBody, firstBody)
	}
}

// Already there is not a failure, and it is not an entry either.
func TestATaskAlreadyInTheTargetStatusIsCarriedAsUnchanged(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	// fill puts every third task in In review, so TAC-1 is there already and TAC-2 is not.
	if got := r.statusOf(t, "TAC-1"); got != domain.StatusInReview {
		t.Fatalf("the fixture put TAC-1 in %q, so this test is checking nothing", got)
	}

	before := r.journalLength(t)
	response := r.post(t, bulkSubmit, token, "bulk-4",
		bulkBody("in_review", map[string]bool{"TAC-1": true, "TAC-2": true}))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the bulk move answered %d: %s", response.StatusCode, r.bodyOf(t, response))
	}

	if got := r.journalLength(t) - before; got != 1 {
		t.Errorf("one task moved and one was already there, and the journal grew by %d", got)
	}
	if got := r.statusOf(t, "TAC-2"); got != domain.StatusInReview {
		t.Errorf("TAC-2 stands in %q; a task standing still must not stop its neighbours", got)
	}

	outcome := decodeBulkAnswer(t, r.bodyOf(t, r.post(t, bulkSubmit, token, "bulk-5",
		bulkBody("in_review", map[string]bool{"TAC-1": true}))))
	if len(outcome.Outcome) != 1 || outcome.Outcome[0].Outcome != "unchanged" {
		t.Errorf("the outcome of a task already in place is %+v, want one entry saying unchanged",
			outcome.Outcome)
	}
}

// One operation, one order — and the order is the numeric one.
//
// The values of a submit arrive as an object, and Go walks a map in a random order, so without a
// sort the entries of one bulk move would land differently on every run. Sorting them as text is
// the trap this checks for: TAC-10 sorts before TAC-2 as a string, and a journal that reads out of
// order is a feed that tells a person a story that did not happen in that sequence.
func TestTheEntriesOfOneBulkMoveComeInNumericOrder(t *testing.T) {
	r := newResource(t)
	r.fill(t, 12)
	token := r.reader(t)

	response := r.post(t, bulkSubmit, token, "bulk-order",
		bulkBody("blocked", map[string]bool{"TAC-2": true, "TAC-10": true, "TAC-11": true}))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the bulk move answered %d", response.StatusCode)
	}
	answer := decodeBulkAnswer(t, r.bodyOf(t, response))

	want := []string{"TAC-2", "TAC-10", "TAC-11"}
	got := make([]string, 0, len(answer.Outcome))
	for _, outcome := range answer.Outcome {
		got = append(got, outcome.Task)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the outcome reads %v, want %v", got, want)
	}

	changes, _, err := r.store.Changes(context.Background(), domain.Start, 500)
	if err != nil {
		t.Fatal(err)
	}
	journal := make([]string, 0, 3)
	for _, change := range changes[len(changes)-3:] {
		journal = append(journal, string(change.Task))
	}
	if strings.Join(journal, ",") != strings.Join(want, ",") {
		t.Errorf("the journal reads %v, want %v", journal, want)
	}
}

// All or nothing. The reason is the idempotency key: a half-applied operation cannot be replayed as
// itself, and the repeat would finish off the remainder instead of answering the same thing.
func TestNothingMovesWhenOneOfTheNamedTasksIsGone(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	before := r.journalLength(t)
	response := r.post(t, bulkSubmit, token, "bulk-6",
		bulkBody("done", map[string]bool{"TAC-2": true, "TAC-404": true}))
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("naming a task that is not there answered %d, want 422: the endpoint was found, "+
			"the payload was not honourable", response.StatusCode)
	}

	if got := r.statusOf(t, "TAC-2"); got == domain.StatusDone {
		t.Error("one task of the batch moved while the batch was refused; the outcome is now partial " +
			"and a repeat under the same key cannot reproduce it")
	}
	if got := r.journalLength(t); got != before {
		t.Errorf("a refused bulk move wrote %d journal entries", got-before)
	}
}

func TestASelectionWithNothingTickedAndAMoveWithNoStatusAreRefused(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	cases := map[string]struct {
		body    string
		message string
	}{
		"nothing ticked": {
			bulkBody("done", map[string]bool{"TAC-1": false}),
			"Tick at least one task before moving them.",
		},
		"no status": {
			bulkBody("", map[string]bool{"TAC-1": true}),
			"Choose where the selected tasks go.",
		},
	}
	for name, c := range cases {
		before := r.journalLength(t)
		response := r.post(t, bulkSubmit, token, "refused-"+name, c.body)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("%s answered %d, want 422", name, response.StatusCode)
		}
		if got := r.journalLength(t); got != before {
			t.Errorf("%s changed something anyway", name)
		}

		// The text, and not only the code. The store refuses both of these on its own, so a test
		// looking at the status alone passes with the handler's checks deleted — and what would be
		// gone is the half a person reads: "domain: invalid task: no tasks were named" is a
		// sentence for a log, and §14 puts finished text on the server.
		var refusal struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(r.bodyOf(t, response), &refusal); err != nil {
			t.Fatal(err)
		}
		if refusal.Error != c.message {
			t.Errorf("%s is refused with %q, want %q", name, refusal.Error, c.message)
		}
	}
}

// The way in.
//
// The selection mode is a screen because the vocabulary has no mode, and a screen nothing links to
// is one nobody reaches: the graph would carry a route that only a client typing its deeplink by
// hand could follow. Which is the same failure as a dead button, seen from the other end.
func TestTheBoardOffersTheWayIntoTheSelectionMode(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)

	response, body := r.get(t, "/screens/board", r.reader(t), "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the board answered %d", response.StatusCode)
	}
	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}

	for _, link := range collectDeeplinks(tree) {
		if link == render.LinkBulkMove {
			return
		}
	}
	t.Errorf("the board leads to %v and to nowhere that moves several tasks at once",
		collectDeeplinks(tree))
}

// The screen: one checkbox per task, every one of them declared, and the count said out loud.
func TestTheSelectionScreenDeclaresACheckboxForEveryTaskItShows(t *testing.T) {
	r := newResource(t)
	r.fill(t, 4)

	form := decodeForm(t, r, bulkForm)

	declared := map[string]bool{}
	for _, field := range form.Schema.Fields {
		declared[field.FieldID] = true
	}
	used := map[string]bool{}
	collectFieldIDs(form.Screen, used)

	if len(used) != 5 {
		t.Errorf("the screen shows %d inputs for four tasks and one selector", len(used))
	}
	for fieldID := range used {
		if !declared[fieldID] {
			t.Errorf("the screen shows %q and the schema does not declare it: nothing can fill it "+
				"and nothing validates it", fieldID)
		}
	}
	for _, id := range []string{"TAC-1", "TAC-2", "TAC-3", "TAC-4"} {
		if !used["task-"+id] {
			t.Errorf("no checkbox names %s, so it cannot be part of a selection", id)
		}
	}
}

// The bound, and the fact that it is spoken. A list that stops without saying so is the failure
// this limit would otherwise introduce — the screen would look like the whole workspace.
func TestTheSelectionScreenSaysHowManyTasksItDoesNotShow(t *testing.T) {
	r := newResource(t)
	r.fill(t, 30)

	form := decodeForm(t, r, bulkForm)

	boxes := 0
	for _, field := range form.Schema.Fields {
		if strings.HasPrefix(field.FieldID, "task-") {
			boxes++
		}
	}
	if boxes != 25 {
		t.Errorf("the screen offers %d checkboxes; the limit is what keeps every field declared "+
			"in the envelope that draws it", boxes)
	}

	if !strings.Contains(textOf(form.Screen), "5 more tasks are not shown") {
		t.Errorf("thirty tasks and twenty-five checkboxes, and the screen does not say the other " +
			"five exist")
	}
}

// The answer is still an action, extension and all: the client runs it through the same chain as
// any other intent, and a body the toolkit refuses to parse would be a screen that never arrives.
func TestTheAnswerOfABulkMoveIsStillAnAction(t *testing.T) {
	r := newResource(t)
	r.fill(t, 2)
	token := r.reader(t)
	v := validator(t)

	body := r.bodyOf(t, r.post(t, bulkSubmit, token, "bulk-7",
		bulkBody("done", map[string]bool{"TAC-1": true})))

	if err := v.Validate(spec.Profile("KompotAction"), body); err != nil {
		t.Errorf("the answer does not satisfy KompotAction: %v", err)
	}

	answer := decodeBulkAnswer(t, body)
	if answer.Type != "navigate" || answer.Deeplink == "" {
		t.Errorf("the answer is %+v, and a submit answers an action", answer)
	}
	if len(answer.Outcome) != 1 || answer.Outcome[0].Task != "TAC-1" || answer.Outcome[0].Outcome != "moved" {
		t.Errorf("the outcome is %+v; B-32 has nowhere to read a partial result from", answer.Outcome)
	}
}

type bulkAnswerBody struct {
	Type     string `json:"type"`
	Deeplink string `json:"deeplink"`
	Outcome  []struct {
		Task    string `json:"task"`
		From    string `json:"from"`
		To      string `json:"to"`
		Outcome string `json:"outcome"`
	} `json:"bulkOutcome"`
}

func decodeBulkAnswer(t *testing.T, body []byte) bulkAnswerBody {
	t.Helper()
	var answer bulkAnswerBody
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("the answer is not JSON: %v", err)
	}
	return answer
}

type formBody struct {
	Schema struct {
		FormID string `json:"formId"`
		Fields []struct {
			Type    string `json:"type"`
			FieldID string `json:"fieldId"`
		} `json:"fields"`
	} `json:"schema"`
	Screen map[string]any `json:"screen"`
}

func decodeForm(t *testing.T, r *resource, path string) formBody {
	t.Helper()

	response, body := r.get(t, path, r.reader(t), "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %d: %s", path, response.StatusCode, body)
	}
	var form formBody
	if err := json.Unmarshal(body, &form); err != nil {
		t.Fatal(err)
	}
	return form
}

// textOf is every string a tree draws, joined. Enough to ask whether something was said, which is
// what these tests want — not where it was said, which is the screenshot's question.
func textOf(node any) string {
	var found []string
	switch value := node.(type) {
	case map[string]any:
		for _, key := range []string{"text", "label", "value", "helperText"} {
			if line, ok := value[key].(string); ok {
				found = append(found, line)
			}
		}
		for _, child := range value {
			found = append(found, textOf(child))
		}
	case []any:
		for _, child := range value {
			found = append(found, textOf(child))
		}
	}
	return strings.Join(found, "\n")
}
