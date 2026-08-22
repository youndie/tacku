package httpsrv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
)

// A task can be changed after it is created, and the journal says which half changed.
//
// Nothing here could do that. The vocabulary of the journal has carried `title_edited` and
// `body_edited` since it was written and the store had no call that produced either — the words for
// what happened were ahead of what could happen, and that gap is invisible to every check, because
// every entry a server writes is one it can write.
func TestEditingATaskRecordsWhichHalfChanged(t *testing.T) {
	r := newResource(t)
	r.fill(t, 1)
	token := r.reader(t)

	task := r.tasks(t)[0]
	before := r.journalLength(t)

	response := r.post(t, "/submit/edit-task/"+string(task.ID), token, "edit-once",
		`{"values":{"title":{"type":"text_value","text":"Rotate the SSO signing keys, again"},`+
			`"description":{"type":"text_value","text":"The third redirect drops the cookie."},`+
			`"due":{"type":"text_value","text":"2026-09-01"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("saving answered %d", response.StatusCode)
	}

	changes, _, err := r.store.Changes(context.Background(), domain.Start, 500)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[domain.ChangeKind]int{}
	for _, change := range changes[before:] {
		kinds[change.Kind]++
	}

	for _, want := range []domain.ChangeKind{domain.ChangeTitleEdited, domain.ChangeBodyEdited, domain.ChangeDueChanged} {
		if kinds[want] != 1 {
			t.Errorf("the journal has %d entries of kind %q, want 1: %v", kinds[want], want, kinds)
		}
	}
}

// Saving a form nobody touched leaves no trace.
//
// A history is read to learn what happened; an entry for a save that changed nothing is an entry
// about somebody opening a screen. The store's own edit refuses to record a change that changed
// nothing, and this is the check that the refusal reaches this far.
func TestSavingAnUntouchedTaskWritesNothing(t *testing.T) {
	r := newResource(t)
	r.fill(t, 1)
	token := r.reader(t)

	task := r.tasks(t)[0]
	body, err := json.Marshal(map[string]any{"values": map[string]any{
		"title":       map[string]any{"type": "text_value", "text": task.Title},
		"description": map[string]any{"type": "text_value", "text": task.Body},
		"due":         map[string]any{"type": "text_value", "text": task.Due},
	}})
	if err != nil {
		t.Fatal(err)
	}

	before := r.journalLength(t)
	r.post(t, "/submit/edit-task/"+string(task.ID), token, "edit-nothing", string(body))

	if added := r.journalLength(t) - before; added != 0 {
		t.Errorf("saving an untouched task wrote %d journal entries", added)
	}
}

// The form arrives carrying what the task currently is.
//
// Through `initialValue` on the field definitions, because the inputs have no value field at all —
// a form that cannot arrive filled in can only create, which is why this screen did not exist.
func TestTheEditFormArrivesFilledIn(t *testing.T) {
	r := newResource(t)
	r.fill(t, 1)
	token := r.reader(t)

	task := r.tasks(t)[0]
	_, body := r.get(t, "/forms/edit-task/"+string(task.ID), token, "")

	var response struct {
		Schema struct {
			Fields []struct {
				FieldID      string          `json:"fieldId"`
				InitialValue json.RawMessage `json:"initialValue"`
			} `json:"fields"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}

	filled := map[string]string{}
	for _, field := range response.Schema.Fields {
		if len(field.InitialValue) > 0 {
			filled[field.FieldID] = string(field.InitialValue)
		}
	}

	if len(filled) == 0 {
		t.Fatal("no field of the edit form carries an initial value: it is a create form wearing another title")
	}
	if !strings.Contains(filled["title"], task.Title) {
		t.Errorf("the title field starts as %q, and the task is called %q", filled["title"], task.Title)
	}
}
