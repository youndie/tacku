package httpsrv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"

	"github.com/youndie/tacku/server/internal/render"
)

// The one token that carries a promise, checked rather than remembered.
//
// `agent` means "a program did this" and nothing else. The moment it paints a decorative element the
// signal stops meaning anything — and it would stop quietly, because a slightly purple box looks
// like a design decision. So every use of it in every tree this server serves has to be a stripe:
// a node three points wide, with a background and no children.
func TestTheAgentColourOnlyEverPaintsAStripe(t *testing.T) {
	r := newResource(t)
	r.fill(t, 4)
	r.agentTouch(t)
	token := r.reader(t)

	screens := []string{
		"/screens/catch-up",
		"/screens/board",
		"/forms/my-tasks",
		"/forms/task/TAC-1",
	}

	uses := 0
	for _, path := range screens {
		response, body := r.get(t, path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s answered %d", path, response.StatusCode)
		}
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Fatal(err)
		}
		uses += checkStripes(t, path, tree)
	}

	// A check that found nothing proves nothing: the trees have to contain the thing being checked.
	if uses == 0 {
		t.Error("no node used the agent colour at all, so this check had nothing to look at")
	}
}

func checkStripes(t *testing.T, path string, node any) int {
	t.Helper()

	value, ok := node.(map[string]any)
	if !ok {
		return 0
	}

	found := 0
	if paintedWith(value, render.ColorAgent) {
		found++
		id, _ := value["id"].(string)

		if width := sizeOf(value); width != render.StripeDp {
			t.Errorf("%s: node %q is painted with the agent colour and is %v points wide, not %d — the provenance signal is being used as decoration",
				path, id, width, render.StripeDp)
		}
		if children, ok := value["children"].([]any); ok && len(children) > 0 {
			t.Errorf("%s: node %q is painted with the agent colour and holds %d children; a stripe holds none",
				path, id, len(children))
		}
	}

	for _, field := range []string{"children", "initialItems"} {
		if children, ok := value[field].([]any); ok {
			for _, child := range children {
				found += checkStripes(t, path, child)
			}
		}
	}
	if child, ok := value["emptyState"].(map[string]any); ok {
		found += checkStripes(t, path, child)
	}
	return found
}

func paintedWith(node map[string]any, token string) bool {
	modifiers, ok := node["modifiers"].([]any)
	if !ok {
		return false
	}
	for _, raw := range modifiers {
		modifier, ok := raw.(map[string]any)
		if ok && modifier["type"] == "background" && modifier["color"] == token {
			return true
		}
	}
	return false
}

func sizeOf(node map[string]any) any {
	modifiers, _ := node["modifiers"].([]any)
	for _, raw := range modifiers {
		modifier, ok := raw.(map[string]any)
		if ok && modifier["type"] == "size" {
			if width, present := modifier["widthDp"]; present {
				if number, ok := width.(float64); ok {
					return int(number)
				}
			}
		}
	}
	return nil
}

// agentTouch makes an agent responsible for something, because a check that finds no agent anywhere
// proves nothing about how an agent is shown.
func (r *resource) agentTouch(t *testing.T) {
	t.Helper()

	boards, err := r.store.Boards(context.Background())
	if err != nil || len(boards) == 0 {
		t.Fatal("no board to work on")
	}
	if _, err := r.store.CreateTask(context.Background(),
		domain.Task{Board: boards[0].ID, Title: "Filed by an agent", Assignee: "anna"},
		domain.Agent("anna-agent", "0.1.0", "anna")); err != nil {
		t.Fatal(err)
	}
}
