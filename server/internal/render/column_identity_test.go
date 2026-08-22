package render_test

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/render"
)

// A column's list is a different node when it holds different cards, and the same one when it does
// not.
//
// Both halves matter and for opposite reasons. Different, because a list shows the items it was
// first given (Q-65): without a new identifier a move lands on the server, the client re-opens the
// board, the new tree arrives correct, and the screen does not change. The same, because the board
// is delivered conditionally — an identifier that moved on its own would change the body on every
// request and no 304 would ever happen again.
func TestAColumnsListIsNamedByWhatItHolds(t *testing.T) {
	todo := domain.Task{ID: "TAC-1", Title: "Fix login redirect loop", Status: domain.StatusTodo}
	other := domain.Task{ID: "TAC-2", Title: "Audit the session cookie flags", Status: domain.StatusTodo}

	before := listIDs(t, render.Board{Title: "Sprint 24", Tasks: []domain.Task{todo, other}}.Screen())
	again := listIDs(t, render.Board{Title: "Sprint 24", Tasks: []domain.Task{todo, other}}.Screen())
	moved := todo
	moved.Status = domain.StatusInProgress
	after := listIDs(t, render.Board{Title: "Sprint 24", Tasks: []domain.Task{moved, other}}.Screen())

	if len(before) == 0 {
		t.Fatal("the board carries no column list at all, so this check looked at nothing")
	}
	if !equal(before, again) {
		t.Errorf("two renderings of the same board named their lists differently:\n%v\n%v", before, again)
	}
	if equal(before, after) {
		t.Errorf("a card moved between columns and every list kept its name: the screen will not change (Q-65)")
	}
}

func listIDs(t *testing.T, screen render.Component) []string {
	t.Helper()

	raw, err := json.Marshal(screen)
	if err != nil {
		t.Fatal(err)
	}

	var found []string
	for _, match := range listID.FindAllStringSubmatch(string(raw), -1) {
		found = append(found, match[1])
	}
	return found
}

var listID = regexp.MustCompile(`"type":"paginated_list","id":"([^"]+)"`)

func equal(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
