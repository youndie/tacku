package render_test

import (
	"encoding/json"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/render"
)

// An update frame names the node it carries, and carries that node once.
//
// A frame says "replace the node with this identifier by this component". Two ways to get that
// wrong, and this product found the second one the hard way:
//
//  1. naming a node that is not the one being sent — the client replaces something else, or nothing;
//  2. sending a component that *contains* a node with the identifier being replaced.
//
// The second is not a wrong picture. Applied when the screen draws, it puts the card inside itself,
// and the next draw does it again: `StackOverflowError` inside recomposition, on the one screen that
// shows cards, the moment anything moves. It arrived when a card grew an outer node for the gap
// between list items while the frame went on naming the inner one.
func TestAnUpdateFrameNamesTheNodeItCarries(t *testing.T) {
	frame := render.CardUpdate(
		domain.Task{ID: "TAC-1", Title: "Fix login redirect loop", Status: domain.StatusTodo},
		domain.Human("anna"),
		"/submit/move",
	)

	raw, err := json.Marshal(frame.Component)
	if err != nil {
		t.Fatal(err)
	}
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}

	rootID, _ := node["id"].(string)
	if rootID != frame.ComponentID {
		t.Errorf("the frame replaces %q and carries a node whose identifier is %q", frame.ComponentID, rootID)
	}

	if inside := countID(node, frame.ComponentID); inside != 1 {
		t.Errorf("the component carries %d nodes identified %q; more than the root means it nests itself when applied",
			inside, frame.ComponentID)
	}
}

func countID(node map[string]any, id string) int {
	found := 0
	if value, _ := node["id"].(string); value == id {
		found++
	}
	for _, field := range []string{"children", "initialItems"} {
		if children, ok := node[field].([]any); ok {
			for _, child := range children {
				if value, ok := child.(map[string]any); ok {
					found += countID(value, id)
				}
			}
		}
	}
	return found
}
