package httpsrv_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
)

// The two ways a person changes a status have to be told apart in the journal, because a product
// decision waits on which of them people use.
//
// The design named the condition before any data existed: if more than half of status changes are
// made after a task has been opened, the card button loses to a plain navigation. Nothing in a
// KOMPOT request names the screen it came from — a perform carries a url and a payload, a submit
// carries a formId the client chose — so the only witness is which address the call arrived at, and
// the entry has to say so at that moment or the answer is gone for good.
//
// This test is not the measurement. It is the thing without which the measurement cannot be taken.
func TestTheJournalSaysWhichSurfaceMovedTheTask(t *testing.T) {
	r := newResource(t)
	token := r.reader(t)
	ctx := context.Background()

	board, err := r.store.CreateBoard(ctx, "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	fromBoard, err := r.store.CreateTask(ctx,
		domain.Task{Board: board.ID, Title: "Moved from the board"}, domain.Human("anna"))
	if err != nil {
		t.Fatal(err)
	}
	fromTask, err := r.store.CreateTask(ctx,
		domain.Task{Board: board.ID, Title: "Moved from the task screen"}, domain.Human("anna"))
	if err != nil {
		t.Fatal(err)
	}

	// The board's card button: a perform on /submit/move carrying the task it acts on.
	response := r.post(t, "/submit/move", token, "move-from-board",
		`{"formId":"board","fieldId":"","values":{`+
			`"task":{"type":"text_value","text":"`+string(fromBoard.ID)+`"},`+
			// Text, and correctly so: a perform carries values the server itself wrote into the
			// action, not values a person chose from a control.
			`"status":{"type":"text_value","text":"in_progress"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the board button answered %d", response.StatusCode)
	}

	// The task screen's selector: a submit of the task form, the task named by the address.
	response = r.post(t, "/submit/task-view/"+string(fromTask.ID), token, "move-from-task",
		`{"formId":"task-view/`+string(fromTask.ID)+`","fieldId":"status","values":{`+
			`"status":{"type":"entity_value","id":"in_progress","title":"in_progress"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the task screen answered %d", response.StatusCode)
	}

	// Counted, not just inspected: a check that finds no targets proves nothing, and two moves that
	// silently wrote nothing would leave every assertion below vacuously true.
	surfaces := map[domain.TaskID]domain.Surface{}
	moves := 0
	changes, _, err := r.store.Changes(ctx, domain.Start, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if change.Kind != domain.ChangeStatusMoved {
			continue
		}
		moves++
		surfaces[change.Task] = change.Surface
	}
	if moves != 2 {
		t.Fatalf("the journal holds %d status moves, want the two just made", moves)
	}

	if got := surfaces[fromBoard.ID]; got != domain.SurfaceBoard {
		t.Errorf("a move made from the board is recorded as %q, want %q", got, domain.SurfaceBoard)
	}
	if got := surfaces[fromTask.ID]; got != domain.SurfaceTask {
		t.Errorf("a move made from the task screen is recorded as %q, want %q", got, domain.SurfaceTask)
	}
}
