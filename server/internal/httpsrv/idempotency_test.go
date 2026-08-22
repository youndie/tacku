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
		`"board":{"type":"entity_value","id":"Sprint 24","title":"Sprint 24"},` +
		`"status":{"type":"entity_value","id":"todo","title":"todo"}}}`
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

// A refusal is remembered, and a corrected request carries a new key.
//
// This test asserted the opposite for most of the life of this repository, on reasoning that read
// well: a request corrected after a refusal has to be able to succeed, or a person who mistyped a
// field is locked out of that attempt for ever. What was missed is that §16.5 already answers it
// from the other side — the client rule is a fresh key per attempt whatever the outcome, so a
// corrected request is a new attempt and brings its own key with it.
//
// Reversed after the question was taken upstream, and reversed here rather than argued with: the
// old behaviour left the hole this harness reported twice as an idempotency defect, where a first
// attempt refused on its merits recorded nothing and the retry with a different body could not be
// told from it.
func TestARefusalIsRememberedAndACorrectedRequestBringsItsOwnKey(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)
	token := r.reader(t)

	empty := `{"formId":"task_create","fieldId":"","values":{"title":{"type":"text_value","text":""}}}`
	if got := r.post(t, "/submit/new-task", token, "retry", empty).StatusCode; got == http.StatusOK {
		t.Fatal("a task with no title was accepted, so this test has nothing to check")
	}

	// The same key with a different body is a conflict, which is what §16.5 calls it.
	reused := r.post(t, "/submit/new-task", token, "retry", newTaskBody("now with a title"))
	if reused.StatusCode != http.StatusConflict {
		t.Errorf("the corrected request under the spent key answered %d, want 409", reused.StatusCode)
	}

	// With its own key it goes through, which is the half that matters to a person.
	fixed := r.post(t, "/submit/new-task", token, "retry-2", newTaskBody("now with a title"))
	if fixed.StatusCode != http.StatusOK {
		t.Errorf("the corrected request with a fresh key answered %d", fixed.StatusCode)
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

// A refusal spends the key, and the same key with a different body afterwards is a conflict.
//
// This is the half that was missing, and its absence is why this repository twice filed a finding
// against itself: a first attempt refused on its merits recorded nothing, so a second attempt under
// the same key found nothing to compare against and was refused for the same reason as the first —
// indistinguishable, from the outside, from an idempotency layer that does not work.
//
// §16.5 settles it through the rule it already had for the client: a fresh key per attempt whatever
// the outcome. A corrected request is a new attempt, so remembering the refusal takes nothing away.
func TestARefusalSpendsTheKeyAndTheNextBodyConflicts(t *testing.T) {
	r := newResource(t)
	r.fill(t, 1)
	token := r.reader(t)

	const key = "spent-by-a-refusal"
	const board = `"board":{"type":"entity_value","id":"Sprint 24","title":"Sprint 24"}`
	refused := r.post(t, "/submit/new-task", token, key,
		`{"formId":"new-task","fieldId":"","values":{`+board+`,"title":{"type":"text_value","text":""}}}`)
	_ = refused.Body.Close()
	if refused.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("the empty submission answered %d, so this test is not looking at a refusal", refused.StatusCode)
	}

	// The same key, a different body. Not a retry of what was refused — a different request, which
	// is exactly what §16.5 calls a conflict.
	conflict := r.post(t, "/submit/new-task", token, key,
		`{"formId":"new-task","fieldId":"","values":{`+board+`,"title":{"type":"text_value","text":"something else"}}}`)
	_ = conflict.Body.Close()
	if conflict.StatusCode != http.StatusConflict {
		t.Errorf("the same key with a different body answered %d, want 409 — a refusal that records "+
			"nothing leaves the second attempt nothing to conflict with", conflict.StatusCode)
	}

	// And the refusal itself replays rather than running again.
	replayed := r.post(t, "/submit/new-task", token, key,
		`{"formId":"new-task","fieldId":"","values":{`+board+`,"title":{"type":"text_value","text":""}}}`)
	_ = replayed.Body.Close()
	if replayed.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("repeating the refused request answered %d, want the refusal back", replayed.StatusCode)
	}
}
