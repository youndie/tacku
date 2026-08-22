package httpsrv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/httpsrv"
	"github.com/youndie/tacku/server/internal/spec"
)

const (
	wizardStart  = "/wizard/new-task"
	wizardResume = "/wizard/new-task/step"
)

// step is what both endpoints of a flow answer while it continues: the wizard_screen and the schema
// beside it.
type step struct {
	Type   string `json:"type"`
	FormID string `json:"formId"`
	Schema struct {
		FormID string `json:"formId"`
		Fields []struct {
			FieldID string `json:"fieldId"`
		} `json:"fields"`
	} `json:"schema"`
	Screen struct {
		Type       string `json:"type"`
		FormID     string `json:"formId"`
		StepID     string `json:"stepId"`
		StepIndex  int    `json:"stepIndex"`
		TotalSteps *int   `json:"totalSteps"`
		CanGoBack  bool   `json:"canGoBack"`
		Content    struct {
			Type string `json:"type"`
		} `json:"content"`
	} `json:"screen"`
}

func (r *resource) startFlow(t *testing.T, token string) (string, step) {
	t.Helper()

	response, body := r.get(t, wizardStart, token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("starting a flow answered %d: %s", response.StatusCode, body)
	}
	scenario := response.Header.Get(httpsrv.WizardHeader)
	if scenario == "" {
		t.Fatalf("wizard_start answered no %s header, so nothing could ever be resumed", httpsrv.WizardHeader)
	}

	var first step
	if err := json.Unmarshal(body, &first); err != nil {
		t.Fatal(err)
	}
	return scenario, first
}

// transition sends one WizardResumeRequest: the transition, the values of the step being left, the
// scenario in its header and an idempotency key.
func (r *resource) transition(t *testing.T, token, scenario, key, body string) (*http.Response, []byte) {
	t.Helper()

	request := r.requestWith(t, http.MethodPost, wizardResume, token, body, map[string]string{
		httpsrv.WizardHeader: scenario,
		"Idempotency-Key":    key,
	})
	return request, r.bodyOf(t, request)
}

// mustTransition is transition for the steps of a walk that are setting up the thing being checked
// rather than being it.
func (r *resource) mustTransition(t *testing.T, token, scenario, key, body string) {
	t.Helper()

	response, answer := r.transition(t, token, scenario, key, body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %d: %s", body, response.StatusCode, answer)
	}
}

func nextWith(values string) string {
	return `{"transition":{"type":"next"},"values":{` + values + `}}`
}

// A board carries its title as its identifier in this store, which is why the two read alike here.
const basicsValues = `"title":{"type":"text_value","text":"Written by a flow"},` +
	`"board":{"type":"entity_value","id":"Sprint 24","title":"Sprint 24"}`

// The first half of the wire B-33 left without a caller: a flow that starts over HTTP, names its
// scenario in a header of this server's choosing (§16.7) and answers a step rather than a form.
func TestAFlowStartsAndNamesItsScenarioInAHeader(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	scenario, first := r.startFlow(t, token)

	if first.Screen.Type != "wizard_screen" {
		t.Errorf("the first step is drawn as %q; without a wizard_screen the client has no progress, "+
			"no Back and no Finish to draw (Q-52)", first.Screen.Type)
	}
	// §11.3: the identifier of the step screen and the identifier of the content's schema must
	// agree, because the client builds its own transitions from it.
	if first.Screen.FormID != first.Schema.FormID {
		t.Errorf("the step names form %q while its schema names %q", first.Screen.FormID, first.Schema.FormID)
	}
	if first.Screen.StepIndex != 0 || first.Screen.CanGoBack {
		t.Errorf("the first step is index %d with canGoBack=%v", first.Screen.StepIndex, first.Screen.CanGoBack)
	}
	if first.Screen.TotalSteps == nil || *first.Screen.TotalSteps != 2 {
		t.Errorf("totalSteps is %v; this walk has a length and must say so", first.Screen.TotalSteps)
	}

	// The header is a name, not a mandate, so the same walk started twice is two walks.
	other, _ := r.startFlow(t, token)
	if other == scenario {
		t.Error("two starts answered the same scenario; starting over has to mean starting over")
	}
}

// The second half: a transition moves the flow, and the answer is an action rather than an envelope
// of its own (§11.5).
func TestATransitionMovesTheFlowForwardAndBack(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	scenario, first := r.startFlow(t, token)

	response, body := r.transition(t, token, scenario, "k-next", nextWith(basicsValues))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("next answered %d: %s", response.StatusCode, body)
	}
	var second step
	if err := json.Unmarshal(body, &second); err != nil {
		t.Fatal(err)
	}
	if second.Type != "wizard_step_result" {
		t.Errorf("a continuing flow answered %q, want wizard_step_result", second.Type)
	}
	if second.Screen.StepID == first.Screen.StepID {
		t.Errorf("next left the flow on step %q", second.Screen.StepID)
	}
	if second.Screen.StepIndex != 1 || !second.Screen.CanGoBack {
		t.Errorf("the second step is index %d with canGoBack=%v", second.Screen.StepIndex, second.Screen.CanGoBack)
	}
	if second.FormID != second.Screen.FormID {
		t.Errorf("the action names form %q and the screen %q", second.FormID, second.Screen.FormID)
	}

	back, body := r.transition(t, token, scenario, "k-back", `{"transition":{"type":"back"},"values":{}}`)
	if back.StatusCode != http.StatusOK {
		t.Fatalf("back answered %d: %s", back.StatusCode, body)
	}
	var again step
	if err := json.Unmarshal(body, &again); err != nil {
		t.Fatal(err)
	}
	if again.Screen.StepID != first.Screen.StepID {
		t.Errorf("back arrived at %q, want %q", again.Screen.StepID, first.Screen.StepID)
	}
	if again.Screen.CanGoBack {
		t.Error("the first step still offers Back, so the history was not popped")
	}
	// The values are held by the server and cannot be shown: no field definition and no input of
	// this profile carries an initial value (Q-53). What the step must not do is pretend otherwise.
	for _, field := range again.Schema.Fields {
		if field.FieldID == "title" {
			return
		}
	}
	t.Error("the step returned to has no title field, so the flow did not come back to where it was")
}

// The transition the server is told about, and the one thing it settles: nothing exists until it
// happens, and it happens once.
func TestFinishCreatesTheTaskOnceAndTheScenarioIsThenGone(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)
	before := len(r.tasks(t))

	scenario, _ := r.startFlow(t, token)
	if response, body := r.transition(t, token, scenario, "k-1", nextWith(basicsValues)); response.StatusCode != http.StatusOK {
		t.Fatalf("next answered %d: %s", response.StatusCode, body)
	}

	response, body := r.transition(t, token, scenario, "k-2", `{"transition":{"type":"finish"},"values":{}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("finish answered %d: %s", response.StatusCode, body)
	}
	var action struct {
		Type     string `json:"type"`
		Deeplink string `json:"deeplink"`
	}
	if err := json.Unmarshal(body, &action); err != nil {
		t.Fatal(err)
	}
	if action.Type != "navigate" {
		t.Errorf("a finished flow answered %q; §11.5 asks for an ordinary action", action.Type)
	}

	tasks := r.tasks(t)
	if len(tasks) != before+1 {
		t.Fatalf("the workspace holds %d tasks, want %d", len(tasks), before+1)
	}

	// A second finish under a fresh key: the scenario is gone, so there is nothing to finish again.
	// This is the half of the protection that does not depend on the idempotency key.
	repeat, _ := r.transition(t, token, scenario, "k-3", `{"transition":{"type":"finish"},"values":{}}`)
	if repeat.StatusCode != http.StatusNotFound {
		t.Errorf("finishing a finished flow answered %d, want 404", repeat.StatusCode)
	}
	if now := r.tasks(t); len(now) != before+1 {
		t.Errorf("the second finish left %d tasks, so it created another one", len(now))
	}
}

// What a step answered a second time does to what was held for it.
//
// The rule is that the fields of the step being left are replaced wholesale: one that did not
// arrive is cleared rather than kept, because a value is left out of a submission when it is empty
// (§9.4) and keeping the old one would make an emptied field impossible to empty. The values of the
// other steps are untouched, which is what carries the first step's answers to the finish.
func TestAStepAnsweredAgainReplacesEverythingItHeld(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)
	before := len(r.tasks(t))

	scenario, _ := r.startFlow(t, token)
	r.mustTransition(t, token, scenario, "k-1", nextWith(basicsValues))
	// A deadline entered on the second step, then left behind by going back.
	r.mustTransition(t, token, scenario, "k-2",
		`{"transition":{"type":"back"},"values":{"due":{"type":"text_value","text":"2026-09-01"}}}`)
	r.mustTransition(t, token, scenario, "k-3", nextWith(basicsValues))
	// The second step, answered again and this time with nothing in it.
	r.mustTransition(t, token, scenario, "k-4", `{"transition":{"type":"finish"},"values":{}}`)

	tasks := r.tasks(t)
	if len(tasks) != before+1 {
		t.Fatalf("the workspace holds %d tasks, want %d", len(tasks), before+1)
	}
	created := tasks[len(tasks)-1]

	if created.Due != "" {
		t.Errorf("the task carries the deadline %q, which was entered and then not entered again", created.Due)
	}
	if created.Title != "Written by a flow" {
		t.Errorf("the task is titled %q; the first step's answers must survive the steps after it", created.Title)
	}
}

// The answer B-33 could only write down, because there was nobody to answer with it. 404 rather
// than 410 and rather than a silent restart, for the reason recorded as Q-25.
func TestAnExpiredScenarioIsAnsweredRatherThanCrashedInto(t *testing.T) {
	const lifetime = 5 * time.Minute

	r := newResourceWith(t, func(config *httpsrv.Config) { config.WizardTTL = lifetime })
	r.seed(t)
	token := r.reader(t)

	scenario, _ := r.startFlow(t, token)

	// The clock moves; the test does not wait. The lifetime is the configured one, so a server
	// still running on the built-in default would answer this transition happily.
	r.clock.pass(lifetime + time.Minute)

	response, body := r.transition(t, token, scenario, "k-late", nextWith(basicsValues))
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("an expired scenario answered %d: %s", response.StatusCode, body)
	}
	var refusal struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &refusal); err != nil {
		t.Fatal(err)
	}
	if refusal.Error == "" {
		t.Error("the refusal carries no text; §16.8 is the only channel this answer has")
	}
}

// The lifetime is configuration and not a constant, which is only demonstrable by two servers
// disagreeing about the same instant.
func TestTheLifetimeComesFromTheConfiguration(t *testing.T) {
	const lifetime = 90 * time.Minute

	r := newResourceWith(t, func(config *httpsrv.Config) { config.WizardTTL = lifetime })
	r.seed(t)
	token := r.reader(t)

	scenario, _ := r.startFlow(t, token)

	// Past the default and well inside the configured lifetime: a server that ignored the setting
	// would have dropped this scenario half an hour ago.
	r.clock.pass(lifetime - time.Minute)

	response, body := r.transition(t, token, scenario, "k-still-here", nextWith(basicsValues))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("a scenario inside its configured lifetime answered %d: %s", response.StatusCode, body)
	}
}

// A scenario identifier is a name rather than a mandate: whoever holds it still has to be the person
// it was minted for.
func TestAScenarioIsNotReachableByTheOtherPerson(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	anna := r.reader(t)
	ivan := r.readerAs(t, "ivan", "ivan@tacku.team", "Ivan")

	scenario, _ := r.startFlow(t, anna)
	r.mustTransition(t, anna, scenario, "k-anna", nextWith(basicsValues))
	before := len(r.tasks(t))

	if response, _ := r.transition(t, ivan, scenario, "k-borrowed", `{"transition":{"type":"back"},"values":{}}`); response.StatusCode != http.StatusNotFound {
		t.Errorf("somebody else's scenario answered %d, want the same 404 an expired one gets", response.StatusCode)
	}

	// And the transition that does something, which is the one the refusal has to come before. A
	// check that only ran on the way out would refuse this call after the task had been created.
	if response, _ := r.transition(t, ivan, scenario, "k-borrowed-finish",
		`{"transition":{"type":"finish"},"values":{}}`); response.StatusCode != http.StatusNotFound {
		t.Errorf("finishing somebody else's flow answered %d, want 404", response.StatusCode)
	}
	if now := r.tasks(t); len(now) != before {
		t.Errorf("the workspace holds %d tasks, want %d: a borrowed scenario created one", len(now), before)
	}
}

// The decision Q-51 records, in the form a caller meets it: the transition is refused without the
// key §16.5 asks of a submit.
func TestATransitionWithoutAnIdempotencyKeyIsRefused(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	scenario, _ := r.startFlow(t, token)

	response, _ := r.transition(t, token, scenario, "", nextWith(basicsValues))
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("a transition without an idempotency key answered %d, want 400", response.StatusCode)
	}
}

// Every transition of the closed hierarchy this flow cannot make, and the one that is not in the
// hierarchy at all. A refusal rather than a panic, and 422 rather than 404: the scenario is alive,
// the request is what is wrong.
func TestATransitionThisFlowCannotMakeIsRefusedOnItsMerits(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	cases := map[string]string{
		"back from the first step": `{"transition":{"type":"back"},"values":{}}`,
		"finish before the end":    `{"transition":{"type":"finish"},"values":{}}`,
		"a jump to a step nobody has been on": `{"transition":{"type":"jump_to","stepId":"details"},` +
			`"values":{}}`,
		"a transition this protocol has no name for": `{"transition":{"type":"cancel"},"values":{}}`,
	}

	for name, body := range cases {
		scenario, _ := r.startFlow(t, token)
		response, answer := r.transition(t, token, scenario, "k-"+name, body)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("%s answered %d, want 422: %s", name, response.StatusCode, answer)
		}
	}
}

// Both answers of the flow against the schema of this build.
//
// The same mitigation the other endpoints have, and it bites harder here: the form hierarchies do
// not degrade, so a wrong wire type inside a step costs the parse of the whole screen.
func TestTheFlowAnswersSatisfyTheSchema(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)
	v := validator(t)

	scenario, _ := r.startFlow(t, token)
	_, started := r.get(t, wizardStart, token, "")
	if err := v.Validate(spec.In("kompot-forms", "KompotFormResponse"), started); err != nil {
		t.Errorf("wizard_start: %v", err)
	}

	response, body := r.transition(t, token, scenario, "k-schema", nextWith(basicsValues))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("next answered %d: %s", response.StatusCode, body)
	}
	if err := v.Validate(spec.Profile("KompotAction"), body); err != nil {
		t.Errorf("wizard_resume: %v", err)
	}
}

// A multi-step scenario needs client code, so it stays out of the navigation graph (§12.1). A route
// added there would promise a client that fetching the address is enough, and the flow would break
// on the first transition with nothing to say why.
func TestTheFlowIsNotOfferedByTheNavigationGraph(t *testing.T) {
	r := newResource(t)
	token := r.reader(t)

	_, body := r.get(t, "/graph", token, "")
	var graph struct {
		Routes []struct {
			Endpoint string `json:"endpoint"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(body, &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Routes) == 0 {
		t.Fatal("the graph offers no route at all, so this check looked at nothing")
	}
	for _, route := range graph.Routes {
		if route.Endpoint == wizardStart || route.Endpoint == wizardResume {
			t.Errorf("the graph offers %s, which needs client code the graph cannot promise", route.Endpoint)
		}
	}
}

func (r *resource) tasks(t *testing.T) []domain.Task {
	t.Helper()

	boards, err := r.store.Boards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var all []domain.Task
	for _, board := range boards {
		tasks, err := r.store.Tasks(context.Background(), board.ID)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, tasks...)
	}
	return all
}
