package httpsrv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
)

func (r *resource) journalLength(t *testing.T) int {
	t.Helper()
	changes, _, err := r.store.Changes(context.Background(), domain.Start, 500)
	if err != nil {
		t.Fatal(err)
	}
	return len(changes)
}

func newTaskBody(title string) string {
	return `{"formId":"task_create","fieldId":"","values":{` +
		`"title":{"type":"text_value","text":"` + title + `"},` +
		`"board":{"type":"text_value","text":"Sprint 24"},` +
		`"status":{"type":"text_value","text":"todo"}}}`
}

// The guarantee B-11 is about, stated as the specification states it: not only that the operation
// does not happen twice, but that nothing beside it does either.
//
// The journal is the side effect this server has today, and it is written inside the same
// transaction as the change — so it would survive a guard placed around the domain call. What it
// would not survive is a handler that writes anything of its own, which is why the count is taken
// rather than the task list.
func TestARepeatedSubmitLeavesNoSecondTrace(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)
	token := r.reader(t)

	before := r.journalLength(t)

	first := r.post(t, "/submit/new-task", token, "attempt-1", newTaskBody("Rotate the SSO signing keys"))
	if first.StatusCode != http.StatusOK {
		t.Fatalf("the first submit answered %d", first.StatusCode)
	}
	afterFirst := r.journalLength(t)
	if afterFirst != before+1 {
		t.Fatalf("one submit wrote %d journal entries", afterFirst-before)
	}

	second := r.post(t, "/submit/new-task", token, "attempt-1", newTaskBody("Rotate the SSO signing keys"))
	if second.StatusCode != http.StatusOK {
		t.Fatalf("the repeat answered %d", second.StatusCode)
	}

	if got := r.journalLength(t); got != afterFirst {
		t.Errorf("the repeat added %d journal entries; a repeat must leave nothing behind", got-afterFirst)
	}
}

// A replay must give back what the first attempt gave, and not merely a plausible answer of the
// same shape: the client feeds it through the same chain as any other intent, so a different
// deeplink would send somebody to a different screen.
func TestARepeatReplaysTheFirstAnswer(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)
	token := r.reader(t)

	read := func(response *http.Response) map[string]any {
		var body map[string]any
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body
	}

	first := read(r.post(t, "/submit/new-task", token, "attempt-2", newTaskBody("Audit the session cookie flags")))
	second := read(r.post(t, "/submit/new-task", token, "attempt-2", newTaskBody("Audit the session cookie flags")))

	if first["deeplink"] != second["deeplink"] {
		t.Errorf("the repeat answered %v where the first answered %v", second["deeplink"], first["deeplink"])
	}
	if first["type"] != "update_session" && first["type"] != "navigate" {
		t.Errorf("a submit answered %v, and a submit answers an action", first["type"])
	}
}

func TestTheSameKeyWithADifferentBodyIsAConflict(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)
	token := r.reader(t)

	if got := r.post(t, "/submit/new-task", token, "shared", newTaskBody("one")).StatusCode; got != http.StatusOK {
		t.Fatalf("the first submit answered %d", got)
	}

	conflict := r.post(t, "/submit/new-task", token, "shared", newTaskBody("something else"))
	if conflict.StatusCode != http.StatusConflict {
		t.Errorf("the same key with a different body answered %d, want 409", conflict.StatusCode)
	}
}

func TestASubmitWithoutAKeyIsRefused(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)

	refused := r.post(t, "/submit/new-task", r.reader(t), "", newTaskBody("no key"))
	if refused.StatusCode != http.StatusBadRequest {
		t.Errorf("a submit without a key answered %d, want 400", refused.StatusCode)
	}
	if r.journalLength(t) != 0 {
		t.Error("a submit refused for want of a key still changed something")
	}
}

// A refusal is not remembered: a request corrected after one has to be able to succeed under the key
// it was refused with, or a client that mistyped a field is locked out of that attempt for ever.
func TestAFailedSubmitDoesNotPoisonItsKey(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)
	token := r.reader(t)

	empty := `{"formId":"task_create","fieldId":"","values":{"title":{"type":"text_value","text":""}}}`
	if got := r.post(t, "/submit/new-task", token, "retry", empty).StatusCode; got == http.StatusOK {
		t.Fatal("a task with no title was accepted, so this test has nothing to check")
	}

	fixed := r.post(t, "/submit/new-task", token, "retry", newTaskBody("now with a title"))
	if fixed.StatusCode != http.StatusOK {
		t.Errorf("the corrected request answered %d under the key its predecessor failed with", fixed.StatusCode)
	}
}

// A read is not a submit. Demanding a key to fetch a screen would be a rule with no failure behind
// it, and would break every client that simply reads.
func TestReadsNeedNoKey(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)

	response, _ := r.get(t, "/screens/catch-up", r.reader(t), "")
	if response.StatusCode != http.StatusOK {
		t.Errorf("reading a screen without an idempotency key answered %d", response.StatusCode)
	}
}

func (r *resource) seedBoard(t *testing.T) {
	t.Helper()
	if _, err := r.store.CreateBoard(context.Background(), "Sprint 24"); err != nil {
		t.Fatal(err)
	}
}
