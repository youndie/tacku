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

// The mark of an agent costs the card nothing.
//
// A card's line is the three facts a board is read for — which task, whose, by when — and for a
// while the byline took their place the moment an agent touched the task: the tasks worked on
// automatically were the ones the board said least about. The failure is silent by construction.
// The line is present either way, the card still looks like a card, and the only way to see it is
// to hold two cards side by side.
//
// Checked on the screens rather than only on the function that writes the line, because a screen is
// where the second way to get this wrong lives: two lists build a card from the same data, and only
// one of them is ever written first. The row of a filtered list once drew a grey stripe for
// everybody for exactly that reason (B-19).
func TestACardAnAgentTouchedKeepsItsIdentifierAndItsDate(t *testing.T) {
	const (
		title = "Filed by an agent"
		due   = "2026-08-29"
		day   = "29 Aug"
	)

	r := newResource(t)
	r.fill(t, 4)

	boards, err := r.store.Boards(context.Background())
	if err != nil || len(boards) == 0 {
		t.Fatal("no board to work on")
	}
	// One task carrying all three facts, with an agent as the last to touch it. Without the due
	// date this check could only ask about the identifier, and the date is the fact the old line
	// dropped most quietly: nobody misses a deadline they were never shown.
	task, err := r.store.CreateTask(context.Background(),
		domain.Task{Board: boards[0].ID, Title: title, Assignee: "anna", Due: due},
		domain.Agent("anna-agent", "0.1.0", "anna"))
	if err != nil {
		t.Fatal(err)
	}

	token := r.reader(t)

	// The board and the filtered list, and not the feed or the task screen: those two carry the
	// byline whole beside the event, which is where the sentence this line drops still lives.
	for _, path := range []string{"/screens/board", "/forms/my-tasks"} {
		response, body := r.get(t, path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s answered %d", path, response.StatusCode)
		}
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Fatal(err)
		}

		// Per screen, because a screen that keeps the promise says nothing about the one beside it,
		// and a summed counter would let it pretend otherwise.
		found := checkAgentCardKeepsItsFacts(t, path, tree, title, string(task.ID), day)
		t.Logf("%s: %d card of %q carrying an agent stripe", path, found, title)
		if found == 0 {
			t.Errorf("%s: an agent filed %q and no card of it on this screen carries a stripe — either the task is not on this screen or this check has nothing to look at",
				path, title)
		}
	}
}

// checkAgentCardKeepsItsFacts returns the number of containers that hold an agent stripe and show
// the named task, complaining about each one that has lost a fact.
//
// The card is found by its title rather than by the shape of its identifier: an identifier is a
// naming convention of this renderer, and a check that depends on one stops looking at the product
// the day the convention changes.
func checkAgentCardKeepsItsFacts(t *testing.T, path string, node any, title, id, day string) int {
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
		if !showsText(value, title) {
			break
		}
		found++
		for _, fact := range []string{id, day, render.AgentWord} {
			if !showsText(value, fact) {
				container, _ := value["id"].(string)
				t.Errorf("%s: node %q carries an agent stripe and no text under it says %q — the mark stands in place of what the card is read for",
					path, container, fact)
			}
		}
		break
	}

	for _, child := range children {
		found += checkAgentCardKeepsItsFacts(t, path, child, title, id, day)
	}
	return found
}

// showsText looks for a fragment in the words of one container, and only in its words: an
// identifier appears in the id of half these nodes, and a check that counted those would pass on a
// card that says nothing at all.
func showsText(node any, fragment string) bool {
	value, ok := node.(map[string]any)
	if !ok {
		return false
	}
	if value["type"] == "text" {
		if body, ok := value["text"].(string); ok && strings.Contains(body, fragment) {
			return true
		}
	}
	for _, child := range descend(value) {
		if showsText(child, fragment) {
			return true
		}
	}
	return false
}
