package httpsrv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
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

	for _, path := range screens {
		response, body := r.get(t, path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s answered %d", path, response.StatusCode)
		}
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Fatal(err)
		}

		// Counted per screen, and it was not always so. Summed across all four, the total was
		// comfortably above zero while one screen — the filtered list — showed a grey stripe for
		// every row including an agent's. A guard against a vacuous check has to be as narrow as
		// the thing it guards, or the screens that keep the promise cover for the one that does
		// not.
		if uses := checkStripes(t, path, tree); uses == 0 {
			t.Errorf("%s: the agent has touched this data and no node on this screen uses the agent colour — either the signal is missing here or this check has nothing to look at",
				path)
		}
	}
}

// The stripe is a colour, and a colour is one channel. This is the check that it is never the only
// one.
//
// Three ways this product says "a program did this", and two of them are colour: the stripe
// (`agent`) and the byline set in `meta_agent`. Counted rather than assumed
// (scripts/token_contrast.py) — `meta_agent` against `meta`, which is the byline a reader compares
// it with, is 1.23:1 in the dark theme and 1.04:1 in the light one, so on a greyscale screen the
// two bylines are the same grey and the second colour is no second channel at all.
//
// What is left is the word. So: wherever a tree carries the stripe, the container holding it must
// also carry text that names the agent — and the container, not the screen, because a byline three
// rows away from the stripe it explains is not a second channel, it is a coincidence.
//
// The one thing this cannot check is whether people read it that way. That half is a test with
// people and is named as a boundary in B-28, not simulated here.
func TestNoAgentStripeIsTheOnlyCarrierOfItsMeaning(t *testing.T) {
	r := newResource(t)
	r.fill(t, 4)
	r.agentTouch(t)
	token := r.reader(t)

	for _, path := range []string{
		"/screens/catch-up",
		"/screens/board",
		"/forms/my-tasks",
		"/forms/task/TAC-1",
	} {
		response, body := r.get(t, path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s answered %d", path, response.StatusCode)
		}
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Fatal(err)
		}

		// Per screen and not summed, for the reason the check above it was written down: four
		// screens keeping the promise cover for the fifth that does not, and the total says nothing
		// about which is which.
		if found := checkStripesAreNamed(t, path, tree); found == 0 {
			t.Errorf("%s: an agent has touched this data and no container on this screen holds a stripe — either the signal is missing here or this check has nothing to look at",
				path)
		}
	}
}

// checkStripesAreNamed returns the number of containers holding an agent stripe, complaining about
// each one whose subtree says nothing in words.
func checkStripesAreNamed(t *testing.T, path string, node any) int {
	t.Helper()

	value, ok := node.(map[string]any)
	if !ok {
		return 0
	}

	found := 0
	children := descend(value)
	for _, child := range children {
		painted, ok := child.(map[string]any)
		if !ok || !paintedWith(painted, render.ColorAgent) {
			continue
		}
		found++
		if !namesTheAgent(value) {
			id, _ := value["id"].(string)
			t.Errorf("%s: node %q holds a stripe in the agent colour and no text under it contains %q — on this screen provenance is carried by colour alone",
				path, id, render.AgentWord)
		}
		break
	}

	for _, child := range children {
		found += checkStripesAreNamed(t, path, child)
	}
	return found
}

// namesTheAgent looks for the word anywhere under one container.
func namesTheAgent(node any) bool {
	value, ok := node.(map[string]any)
	if !ok {
		return false
	}
	if value["type"] == "text" {
		if body, ok := value["text"].(string); ok && strings.Contains(body, render.AgentWord) {
			return true
		}
	}
	for _, child := range descend(value) {
		if namesTheAgent(child) {
			return true
		}
	}
	return false
}

// descend is every child a node can have, whatever the field is called. "screen" is in the list
// because a form answers `{schema, screen}` and everything worth walking hangs off the second half.
func descend(node map[string]any) []any {
	children := make([]any, 0, 4)
	for _, field := range []string{"children", "initialItems"} {
		if list, ok := node[field].([]any); ok {
			children = append(children, list...)
		}
	}
	for _, field := range []string{"emptyState", "screen"} {
		if child, ok := node[field].(map[string]any); ok {
			children = append(children, child)
		}
	}
	return children
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
	// "screen" because a form answers `{schema, screen}` and the tree hangs off the second half.
	// Without it this walk stopped at the envelope: two of the four screens listed above were never
	// looked at, and the check reported on them anyway.
	for _, field := range []string{"emptyState", "screen"} {
		if child, ok := value[field].(map[string]any); ok {
			found += checkStripes(t, path, child)
		}
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
	// And an agent moving something that already existed, so the screen of one task has an agent
	// in its history too. Without it that screen could only be checked for the absence of a wrong
	// colour, which is not the same as checking that the right one appears.
	if _, err := r.store.MoveTask(context.Background(), "TAC-1", domain.StatusDone,
		domain.Agent("anna-agent", "0.1.0", "anna"), domain.SurfaceAgent); err != nil {
		t.Fatal(err)
	}
}

// The signal has to survive scrolling.
//
// The first page of a list and its continuation are two different handlers answering one address,
// and only one of them was passing provenance through when this was written. A stripe that appears
// above the fold and disappears below it is worse than one that is missing everywhere: it teaches
// the reader that grey means a person, and then quietly lies.
func TestProvenanceSurvivesIntoTheContinuationPage(t *testing.T) {
	r := newResource(t)
	r.fill(t, 12)
	token := r.reader(t)

	// Filed after the first page's worth, so the agent's task is only reachable by continuing.
	boards, err := r.store.Boards(context.Background())
	if err != nil || len(boards) == 0 {
		t.Fatal("no board to work on")
	}
	if _, err := r.store.CreateTask(context.Background(),
		domain.Task{Board: boards[0].ID, Title: "Filed by an agent", Assignee: "anna"},
		domain.Agent("anna-agent", "0.1.0", "anna")); err != nil {
		t.Fatal(err)
	}

	first := r.page(t, "/pages/tasks", token)
	if first.NextLoadAction == nil {
		t.Fatal("the first page offered no continuation, so this test has nothing to check")
	}

	found := 0
	path := first.NextLoadAction.URL
	pages := 0
	for range 20 {
		current := r.page(t, path, token)
		pages++
		for _, item := range current.Items {
			found += checkStripes(t, "continuation", item)
		}
		if current.NextLoadAction == nil {
			break
		}
		path = current.NextLoadAction.URL
	}

	if pages == 0 {
		t.Fatal("no continuation was read")
	}
	if found == 0 {
		t.Error("an agent filed a task that only appears past the first page, and no row after the first page carries the agent colour")
	}
}
