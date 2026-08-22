package httpsrv_test

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

// The boundary of B-27 through the wire, which is where the two halves of the answer meet: the rule
// lives in the domain, but whether the screen ever calls it — and with what — only a request shows.
//
// One story rather than five tests, because the interesting property is a sequence: what a person
// sees on a reload has to depend on what they saw a moment ago, and a test that arrives once cannot
// tell a boundary that never moves from one that moves too eagerly.
func TestTheFeedIsMeasuredFromTheEndOfThePreviousVisit(t *testing.T) {
	r := newResource(t)
	board := r.board(t)
	token := r.reader(t)

	r.agentFiles(t, board, 3)

	// First visit: never been here, so everything is news.
	if got := r.feedRows(t, token); got != 3 {
		t.Fatalf("the first visit shows %d changes, want 3 — everything that ever happened", got)
	}

	// A reload two minutes later is the same visit. The screen must not empty under the reader,
	// and what arrived meanwhile belongs to this visit too.
	r.agentFiles(t, board, 2)
	r.clock.pass(2 * time.Minute)
	if got := r.feedRows(t, token); got != 5 {
		t.Fatalf("a reload inside the visit shows %d changes, want 5", got)
	}

	// A night away. Everything the previous visit was offered is behind the boundary now.
	r.clock.pass(9 * time.Hour)
	if got := r.feedRows(t, token); got != 0 {
		t.Fatalf("the morning after shows %d changes, want 0 — all five were offered yesterday", got)
	}

	// And the new visit still catches what happened while it was away from the screen.
	r.agentFiles(t, board, 1)
	r.clock.pass(time.Minute)
	if got := r.feedRows(t, token); got != 1 {
		t.Fatalf("one change arrived during the visit and the feed shows %d", got)
	}
}

// Changes that land in the gap between two visits belong to the second one.
//
// The failure this rules out is the tempting shortcut: advancing the boundary to the end of the
// journal on arrival. Everything above still passes under it — the difference only shows for a
// change written after the visit was last seen and before the next one begins.
func TestWhatHappenedBetweenTwoVisitsIsShownInTheSecond(t *testing.T) {
	r := newResource(t)
	board := r.board(t)
	token := r.reader(t)

	r.agentFiles(t, board, 2)
	if got := r.feedRows(t, token); got != 2 {
		t.Fatalf("the first visit shows %d changes, want 2", got)
	}

	// Away, and the agent works while nobody is looking.
	r.clock.pass(9 * time.Hour)
	r.agentFiles(t, board, 4)

	if got := r.feedRows(t, token); got != 4 {
		t.Fatalf("the second visit shows %d changes, want exactly the 4 that happened while away", got)
	}
}

// The headline says how long, and says it as elapsed time.
//
// Not the date and time the mockup drew: §14 hands the server the finished text and §16.7 gives it
// no timezone, so a wall clock here would be the server's own wearing the reader's (Q-25).
func TestTheHeadlineNamesHowLongTheReaderWasAway(t *testing.T) {
	r := newResource(t)
	board := r.board(t)
	token := r.reader(t)

	_, first := r.get(t, "/screens/catch-up", token, "")
	if strings.Contains(headline(t, first), "last here") {
		t.Errorf("a first visit was told when it was last here: %q", headline(t, first))
	}

	r.clock.pass(9 * time.Hour)
	r.agentFiles(t, board, 1)
	_, second := r.get(t, "/screens/catch-up", token, "")

	const want = "1 change across 1 board · you were last here 9 hours ago"
	if headline(t, second) != want {
		t.Errorf("the headline says %q, want %q", headline(t, second), want)
	}

	// And it stays put for the whole visit rather than counting up against the clock: a sentence
	// recomputed every hour would change the body every hour and cost the 304 for nothing.
	r.clock.pass(3 * time.Hour)
	_, later := r.get(t, "/screens/catch-up", token, "")
	if headline(t, later) != want {
		t.Errorf("three hours into the visit the headline says %q, want %q", headline(t, later), want)
	}
}

// Pressing the button has to survive walking away.
//
// The boundary the button sets and the position the next visit starts from are two values, and the
// half that is easy to forget is the second: left behind, it would rewind the boundary to where the
// previous visit ended the next morning, and the whole feed would come back after it was dismissed.
func TestAnExplicitMarkIsNotUndoneByTheNextVisit(t *testing.T) {
	r := newResource(t)
	board := r.board(t)
	token := r.reader(t)

	r.agentFiles(t, board, 3)
	if got := r.feedRows(t, token); got != 3 {
		t.Fatalf("the first visit shows %d changes, want 3", got)
	}

	// Two more land after the screen was last drawn, so that "all" means more than what the visit
	// had been offered. Without them the arrival has already moved the pending position to the end
	// on its own, and the button could get away with moving only half the boundary.
	r.agentFiles(t, board, 2)

	response := r.post(t, "/submit/seen", token, "mark-1", `{"formId":"","fieldId":"","values":{}}`)
	if response.StatusCode != 200 {
		t.Fatalf("marking everything seen answered %d", response.StatusCode)
	}

	r.clock.pass(9 * time.Hour)
	if got := r.feedRows(t, token); got != 0 {
		t.Fatalf("the morning after a mark shows %d changes, want 0", got)
	}

	// And the mark is stamped by the same clock the arrival is measured against. Two clocks that
	// are compared to each other are one clock with a bug in it, and the symptom is not the
	// boundary — it is this sentence, which counts from whichever moment the mark recorded.
	_, body := r.get(t, "/screens/catch-up", token, "")
	const want = "0 changes across 0 boards · you were last here 9 hours ago"
	if headline(t, body) != want {
		t.Errorf("the headline says %q, want %q", headline(t, body), want)
	}
}

// board is a workspace with nothing in it yet, so that a test can decide when the journal starts.
func (r *resource) board(t *testing.T) domain.BoardID {
	t.Helper()

	board, err := r.store.CreateBoard(context.Background(), "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	return board.ID
}

// agentFiles writes count entries into the journal the way the agent does.
func (r *resource) agentFiles(t *testing.T, board domain.BoardID, count int) {
	t.Helper()

	for range count {
		if _, err := r.store.CreateTask(context.Background(),
			domain.Task{Board: board, Title: "filed by the agent"},
			domain.Agent("anna-agent", "0.1.0", "anna")); err != nil {
			t.Fatal(err)
		}
	}
}

// feedRows is how many entries the catch-up screen actually shows.
//
// Counted from the rendered tree rather than from the headline, which is a separate number computed
// a separate way: a test that read the headline would agree with itself while the list beside it
// showed something else.
func (r *resource) feedRows(t *testing.T, token string) int {
	t.Helper()

	_, body := r.get(t, "/screens/catch-up", token, "")

	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}
	ids := map[string]int{}
	walk(tree, ids)

	rows := 0
	for id := range ids {
		if rowID.MatchString(id) {
			rows++
		}
	}
	return rows
}

// The identifier of a row and not of the nodes inside it: those are derived from it by suffix, and
// counting them would multiply every answer by the shape of the row.
var rowID = regexp.MustCompile(`^change-[0-9]+$`)
