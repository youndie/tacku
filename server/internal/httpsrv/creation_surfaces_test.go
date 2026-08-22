package httpsrv_test

import (
	"encoding/json"
	"slices"
	"testing"
)

// The two ways of creating a task ask for the same facts.
//
// They arrived from different directions — a one-screen form and a scenario — and diverged the
// moment one of them grew a field. Nothing failed: a task simply came out of one path without a
// description, and no test of either path could see it, because each looked only at its own.
//
// The rule is not symmetry for its own sake. Two paths that take different facts make one of them
// the path that loses something, and which one that is has to be a decision somebody wrote down
// rather than a difference nobody noticed.
func TestBothWaysOfCreatingATaskAskForTheSameFacts(t *testing.T) {
	r := newResource(t)
	r.fill(t, 1)
	token := r.reader(t)

	form := fieldsOf(t, r, token, "/forms/new-task")

	// The scenario is walked rather than fetched twice: its second step is reached by a transition
	// and by nothing else, and asking the start address again would have compared the first step
	// with itself — a comparison that passes while saying nothing.
	id, first := r.startFlow(t, token)
	_, second := r.transition(t, token, id, "surfaces", nextWith(basicsValues))
	scenario := append(declaredBy(first), declaredBy(decodeStep(t, second))...)

	if len(form) == 0 || len(scenario) == 0 {
		t.Fatalf("one of the surfaces declared no field at all (form %v, scenario %v), so this comparison proves nothing",
			form, scenario)
	}

	slices.Sort(form)
	slices.Sort(scenario)
	if !slices.Equal(form, scenario) {
		t.Errorf("the form asks for %v and the scenario for %v; a task created one way loses what the other collects",
			form, scenario)
	}
}

// declaredBy reads the field identifiers of a step of a flow.
func declaredBy(one step) []string {
	found := make([]string, 0, len(one.Schema.Fields))
	for _, field := range one.Schema.Fields {
		found = append(found, field.FieldID)
	}
	return found
}

func decodeStep(t *testing.T, body []byte) step {
	t.Helper()

	var one step
	if err := json.Unmarshal(body, &one); err != nil {
		t.Fatalf("the step after a transition did not decode: %v", err)
	}
	return one
}

// fieldsOf reads the field identifiers a form response declares.
func fieldsOf(t *testing.T, r *resource, token, path string) []string {
	t.Helper()

	response, body := r.get(t, path, token, "")
	if response.StatusCode != 200 {
		t.Fatalf("%s answered %d: %s", path, response.StatusCode, body)
	}

	var envelope struct {
		Schema struct {
			Fields []struct {
				FieldID string `json:"fieldId"`
			} `json:"fields"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	found := make([]string, 0, len(envelope.Schema.Fields))
	for _, field := range envelope.Schema.Fields {
		found = append(found, field.FieldID)
	}
	return found
}
