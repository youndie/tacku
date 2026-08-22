package httpsrv_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
)

// One press finishes a task, and the journal says so once.
//
// The status chain is linear and forward, so the commonest outcome — a task is done — cost two
// intermediate moves from To do. Each was a request, and each wrote an entry about a transition
// nobody intended; a history read afterwards described the mechanism rather than the work.
//
// The address matters as much as the count. It is the task's own, not the board's, because the
// surface is derived from the address and the measurement of where people change status (B-36) is
// derived from the surface. Posting this to the board's endpoint would have worked and quietly
// spoiled the number.
func TestMarkingDoneCostsOnePressAndOneEntry(t *testing.T) {
	r := newResource(t)
	r.fill(t, 1)
	token := r.reader(t)

	tasks := r.tasks(t)
	if len(tasks) == 0 {
		t.Fatal("the workspace carries no task, so this check pressed nothing")
	}
	task := tasks[0]

	before := r.journalLength(t)

	response := r.post(t, "/submit/task-view/"+string(task.ID), token, "done-once",
		`{"values":{"status":{"type":"entity_value","id":"done"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("marking done answered %d", response.StatusCode)
	}

	if added := r.journalLength(t) - before; added != 1 {
		t.Errorf("marking a task done wrote %d journal entries, not 1: the history describes the mechanism rather than the work", added)
	}
	if got := r.statusOf(t, task.ID); got != domain.StatusDone {
		t.Errorf("the task is %q after being marked done", got)
	}

	changes, _, err := r.store.Changes(context.Background(), domain.Start, 500)
	if err != nil {
		t.Fatal(err)
	}
	latest := changes[len(changes)-1]
	if latest.Surface != domain.SurfaceTask {
		t.Errorf("the entry says surface %q; from this screen it is %q, and B-36 counts by it",
			latest.Surface, domain.SurfaceTask)
	}
}

// The button is absent on a task already there.
//
// A control that does nothing invites a press and answers with silence, which reads as the product
// being broken rather than the task being finished.
func TestAFinishedTaskOffersNoMarkAsDone(t *testing.T) {
	r := newResource(t)
	r.fill(t, 1)
	token := r.reader(t)

	task := r.tasks(t)[0]

	_, before := r.get(t, "/forms/task/"+string(task.ID), token, "")
	if !strings.Contains(string(before), "Mark as Done") {
		t.Fatal("an unfinished task does not offer to mark it done, so the next assertion proves nothing")
	}

	r.post(t, "/submit/task-view/"+string(task.ID), token, "done-first",
		`{"values":{"status":{"type":"entity_value","id":"done"}}}`)

	_, after := r.get(t, "/forms/task/"+string(task.ID), token, "")
	if strings.Contains(string(after), "Mark as Done") {
		t.Error("a task already in Done still offers to mark it done")
	}
}
