package wizard_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/wizard"
)

const (
	anna = domain.MemberID("anna")
	ivan = domain.MemberID("ivan")

	form = "task_create"
	ttl  = 30 * time.Minute
)

// clock is what makes these tests about the code rather than about the machine they run on. A test
// that proved expiry by sleeping for the lifetime would measure the test runner, and would have to
// choose between a lifetime too short to be the real one and a run too slow to keep.
type clock struct{ at time.Time }

func (c *clock) now() time.Time { return c.at }

func (c *clock) advance(d time.Duration) { c.at = c.at.Add(d) }

func store(t *testing.T) (*wizard.Store, *clock) {
	t.Helper()
	c := &clock{at: time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)}
	return wizard.New(ttl, c.now), c
}

func start(t *testing.T, s *wizard.Store, owner domain.MemberID) wizard.ID {
	t.Helper()
	id, err := s.Start(owner, form, "title")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func text(value string) json.RawMessage {
	raw, _ := json.Marshal(map[string]string{"type": "text_value", "text": value})
	return raw
}

func TestAnAbandonedScenarioIsGoneOnceItsLifetimeRuns(t *testing.T) {
	s, c := store(t)
	id := start(t, s, anna)

	c.advance(ttl - time.Second)
	if _, err := s.Resume(id, anna); err != nil {
		t.Fatalf("a second before the deadline the flow must still continue: %v", err)
	}

	// Nothing touches it from here: this is what walking away looks like from the server's side,
	// there being no cancel transition to hear (Q-11).
	c.advance(ttl)
	if _, err := s.Resume(id, anna); !errors.Is(err, wizard.ErrGone) {
		t.Fatalf("an abandoned flow must be gone, got %v", err)
	}
}

func TestExpiredStateIsRemovedRatherThanHidden(t *testing.T) {
	s, c := store(t)
	start(t, s, anna)
	start(t, s, ivan)

	c.advance(ttl)

	if swept := s.Sweep(); swept != 2 {
		t.Fatalf("both abandoned flows must be swept, got %d", swept)
	}
	// Counted rather than read back: an entry that Resume refuses but the map still holds is a leak
	// that reads exactly like a fixed one.
	if held := s.Len(); held != 0 {
		t.Fatalf("expired state must be released, %d entries still held", held)
	}
}

// The two tests below are deliberately not one walk that both reads and writes. A walk that did
// both would stay green with either renewal removed, because the other one would carry it — two
// guarantees checked by one test is one guarantee checked twice.

func TestReadingAStepKeepsTheFlowAlive(t *testing.T) {
	s, c := store(t)
	id := start(t, s, anna)

	// Two pauses, each a minute short of the lifetime, add up to more than it. They must not add up
	// to an expiry: what is collected here is state nobody came back to, and a person who keeps
	// coming back is the opposite of that.
	for step := 0; step < 2; step++ {
		c.advance(ttl - time.Minute)
		if _, err := s.Resume(id, anna); err != nil {
			t.Fatalf("step %d: a flow in use must not expire under the person filling it: %v", step, err)
		}
	}
}

func TestWritingAStepKeepsTheFlowAlive(t *testing.T) {
	s, c := store(t)
	id := start(t, s, anna)

	c.advance(ttl - time.Minute)
	if err := s.Save(id, anna, wizard.State{Step: "board", History: []string{"title"}}); err != nil {
		t.Fatalf("a step saved a minute before the deadline must land: %v", err)
	}

	c.advance(ttl - time.Minute)
	state, err := s.Resume(id, anna)
	if err != nil {
		t.Fatalf("the saved step must have moved the deadline with it: %v", err)
	}
	if state.Step != "board" || len(state.History) != 1 {
		t.Fatalf("the step and the history of the walk must survive with it, got %q %v", state.Step, state.History)
	}
}

func TestStartingOverBeginsBlank(t *testing.T) {
	s, _ := store(t)

	first := start(t, s, anna)
	state, err := s.Resume(first, anna)
	if err != nil {
		t.Fatal(err)
	}
	state.Values["title"] = text("Draft nobody meant to keep")
	if err := s.Save(first, anna, state); err != nil {
		t.Fatal(err)
	}

	// Walked away from, then walked in again — with no cancel to send, this is the only shape the
	// server ever sees of "start over".
	second := start(t, s, anna)
	if second == first {
		t.Fatal("starting over must mint a new identifier, or the held state surfaces in the next walk")
	}

	fresh, err := s.Resume(second, anna)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Values) != 0 {
		t.Fatalf("a new walk must start blank, got %v", fresh.Values)
	}
	if fresh.Step != "title" || len(fresh.History) != 0 {
		t.Fatalf("a new walk must start at the first step with no history, got %q %v", fresh.Step, fresh.History)
	}
}

func TestFinishDropsTheStateOnTheSignalThereIs(t *testing.T) {
	s, _ := store(t)
	id := start(t, s, anna)

	if err := s.Finish(id, anna); err != nil {
		t.Fatal(err)
	}
	if held := s.Len(); held != 0 {
		t.Fatalf("a finished flow must not wait for the clock, %d entries still held", held)
	}
	if _, err := s.Resume(id, anna); !errors.Is(err, wizard.ErrGone) {
		t.Fatalf("a finished flow cannot be continued, got %v", err)
	}
}

func TestStartingAFlowReleasesTheAbandonedOnes(t *testing.T) {
	s, c := store(t)
	start(t, s, anna)

	c.advance(ttl)
	start(t, s, anna)

	// The map only grows where flows are started, so that is where it has to shrink: without this,
	// a server nobody sweeps holds every abandoned walk until it restarts.
	if held := s.Len(); held != 1 {
		t.Fatalf("starting a flow must release the expired ones, %d entries held", held)
	}
}

func TestAScenarioIdentifierIsNotAMandate(t *testing.T) {
	s, _ := store(t)
	id := start(t, s, anna)

	if _, err := s.Resume(id, ivan); !errors.Is(err, wizard.ErrGone) {
		t.Fatalf("a handle names a flow, it does not authorise carrying it, got %v", err)
	}
	if err := s.Finish(id, ivan); !errors.Is(err, wizard.ErrGone) {
		t.Fatalf("nor does it authorise ending it, got %v", err)
	}
	// And the refusal must not have taken the flow away from the person whose it is.
	if _, err := s.Resume(id, anna); err != nil {
		t.Fatalf("the owner's flow must survive somebody else's attempt: %v", err)
	}
}

func TestStateHandedOutIsACopy(t *testing.T) {
	s, _ := store(t)
	id := start(t, s, anna)

	state, err := s.Resume(id, anna)
	if err != nil {
		t.Fatal(err)
	}
	state.Values["title"] = text("Never saved")
	state.History = append(state.History, "title")

	stored, err := s.Resume(id, anna)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Values) != 0 || len(stored.History) != 0 {
		t.Fatalf("a value must reach the store through Save and no other way, got %v %v", stored.Values, stored.History)
	}
}

func TestOwnerAndFormAreNotTheCallersToChange(t *testing.T) {
	s, _ := store(t)
	id := start(t, s, anna)

	state, err := s.Resume(id, anna)
	if err != nil {
		t.Fatal(err)
	}
	state.Owner = ivan
	state.Form = "board_delete"
	if err := s.Save(id, anna, state); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Resume(id, ivan); !errors.Is(err, wizard.ErrGone) {
		t.Fatalf("a save must not hand the flow to somebody else, got %v", err)
	}
	stored, err := s.Resume(id, anna)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Form != form {
		t.Fatalf("a save must not turn the flow into another one, got %q", stored.Form)
	}
}

func TestTheLifetimeIsConfiguredRatherThanBuiltIn(t *testing.T) {
	c := &clock{at: time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)}
	short := wizard.New(time.Minute, c.now)

	id, err := short.Start(anna, form, "title")
	if err != nil {
		t.Fatal(err)
	}
	c.advance(time.Minute)
	if _, err := short.Resume(id, anna); !errors.Is(err, wizard.ErrGone) {
		t.Fatalf("the configured minute must be the one that expires, got %v", err)
	}

	if wizard.New(0, c.now).TTL() != wizard.DefaultTTL {
		t.Fatal("an unset lifetime must fall back to the documented default, not to zero")
	}
}
