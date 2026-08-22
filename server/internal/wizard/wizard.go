// Package wizard holds the state of a multi-step flow between two requests.
//
// The protocol leaves the server no choice about holding it. A resume request carries a transition
// and the values of the step just filled in (`WizardResumeRequest`); a back transition carries
// nothing but its own type; the scenario is named by a header of its own (SPEC.md §16.7, §11.4).
// The client never sends the step, the history or anything entered earlier, so "Back" has nowhere
// to go unless the server remembers — and under branching, where `totalSteps` is null, not even the
// length of the walk is known in advance.
//
// It leaves the server no way to be told the flow is over, either. The transitions are `next`,
// `back`, `finish` and `jump_to`; there is no cancel (Q-11). A person who walks away presses a
// button the client handles on its own, and this package never hears about it. So a finished
// scenario is dropped on its own signal, and an abandoned one can only be dropped by a clock —
// which makes the lifetime here a consequence of the protocol rather than a product decision
// (Q-25).
//
// The state is invisible to the person and says nothing about the promise the interface makes.
// "Nothing is saved until you finish" is about the task: none is created, and nothing appears
// anywhere. Confusing the two is what makes this look like a contradiction.
package wizard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

// DefaultTTL is how long an untouched scenario is kept.
//
// A choice, not a measurement: nobody has watched people fill in a step, and the number that would
// settle this is how long a real interruption lasts. What is known is what the value has to sit
// between. Below, a person who answers the phone on step two must come back to their step, so
// minutes rather than seconds. Above, state nobody can cancel must not be measured in days, and
// the access token this server issues lives fifteen minutes and is refreshed silently, so half an
// hour does not outlive a session in any way the person would notice.
//
// Whoever measures it changes this one constant and nothing else — the store takes the lifetime as
// an argument precisely so the number lives in configuration and not in the logic.
const DefaultTTL = 30 * time.Minute

// ErrGone is the answer to a scenario that has expired, never existed, or belongs to somebody else.
//
// One error for the three on purpose. The three are the same fact for the person in front of the
// screen — this flow cannot be continued, start it again — and telling them apart would let anyone
// holding a wrong identifier learn whether it is a real one.
var ErrGone = errors.New("wizard: this flow cannot be continued; start it again")

// ID names a scenario. It travels in a header of the wizard_start response and comes back on every
// resume, so it is a handle: a name, never a mandate. Owner is checked on every call.
type ID string

// State is what SPEC.md §13.1 says is kept between requests: the current step, the history and the
// values accumulated so far.
type State struct {
	// Form is the formId of the flow, which the step screen and its transitions must agree on
	// (§11.3).
	Form string
	// Owner is the member who started the flow.
	Owner domain.MemberID
	// Step is the step being shown now.
	Step string
	// History is the steps walked to reach it, oldest first. Back pops it; under branching this is
	// the only record of the route taken.
	History []string
	// Values are the fields entered so far, keyed by fieldId, in the shape they arrived in.
	Values map[string]json.RawMessage
}

type entry struct {
	state State
	// deadline moves forward on every touch. What is being removed is an *abandoned* scenario, and
	// abandonment shows up as nothing happening; a fixed deadline would instead delete the work of
	// somebody still typing at minute thirty-one of a long form.
	deadline time.Time
}

// Store keeps live scenarios in memory.
//
// In memory rather than in SQLite, and the reason is that this state is not a fact about the
// workspace: it belongs to one person's unfinished walk through one screen, it is unreadable to
// everything else in the product, and a restart losing it costs exactly what the expiry above
// costs. Writing it to the database would put rows nobody can cancel next to rows that are the
// product's actual record. When a second process appears this becomes a real decision rather than
// this note — a flow started on one instance would then have to resume on another.
type Store struct {
	mu   sync.Mutex
	live map[ID]*entry
	ttl  time.Duration
	now  func() time.Time
}

// New builds a store whose scenarios live for ttl without being touched.
//
// The clock is a parameter because the guarantee is about time passing, and a test that proves it
// by sleeping measures the test runner. Pass nil outside tests.
func New(ttl time.Duration, now func() time.Time) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Store{live: map[ID]*entry{}, ttl: ttl, now: now}
}

// TTL is the lifetime this store was configured with.
func (s *Store) TTL() time.Duration { return s.ttl }

// Start opens a scenario and answers the identifier the client will come back with.
//
// Always a new identifier, never one handed out before: starting over has to mean starting over.
// Reusing an identifier per person and form would let values held from a walk that was abandoned
// surface in the next one — silently, and against the only thing the screen promised.
func (s *Store) Start(owner domain.MemberID, form, step string) (ID, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Swept here rather than by a goroutine with a ticker: the map only grows when a flow is
	// started, so sweeping where it grows bounds it without a second thing to start, stop and
	// shut down.
	s.sweepLocked()

	s.live[id] = &entry{
		state:    State{Form: form, Owner: owner, Step: step, Values: map[string]json.RawMessage{}},
		deadline: s.now().Add(s.ttl),
	}
	return id, nil
}

// Resume reads a live scenario and pushes its deadline out.
func (s *Store) Resume(id ID, owner domain.MemberID) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	found, err := s.liveLocked(id, owner)
	if err != nil {
		return State{}, err
	}
	found.deadline = s.now().Add(s.ttl)
	return copyState(found.state), nil
}

// Save writes the step back and pushes the deadline out.
func (s *Store) Save(id ID, owner domain.MemberID, state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found, err := s.liveLocked(id, owner)
	if err != nil {
		return err
	}
	// The owner and the form are not the caller's to change: a resume that renamed either would be
	// a different scenario wearing this one's identifier.
	state.Owner = found.state.Owner
	state.Form = found.state.Form
	found.state = copyState(state)
	found.deadline = s.now().Add(s.ttl)
	return nil
}

// Finish drops a scenario that reached its end.
//
// The one ending the server is told about, so it is the one that costs nothing to detect. Only
// abandonment falls through to the clock.
func (s *Store) Finish(id ID, owner domain.MemberID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.liveLocked(id, owner); err != nil {
		return err
	}
	delete(s.live, id)
	return nil
}

// Sweep drops everything past its deadline and answers how many went. Exported for whatever ends up
// wanting to run it on a timer, and for a test to count with.
func (s *Store) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sweepLocked()
}

// Len is how many scenarios are held, expired ones included. A test asserting that abandoned state
// is gone has to count what is *kept*: an expired entry that is merely hidden from Resume is still
// a leak, and reads exactly like a fixed one.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.live)
}

func (s *Store) sweepLocked() int {
	now := s.now()
	swept := 0
	for id, held := range s.live {
		if !now.Before(held.deadline) {
			delete(s.live, id)
			swept++
		}
	}
	return swept
}

// liveLocked finds a scenario that is present, unexpired and the caller's.
//
// An expired one is deleted here rather than stepped over, so that a client polling a dead
// identifier cannot keep it in the map for ever.
func (s *Store) liveLocked(id ID, owner domain.MemberID) (*entry, error) {
	held, ok := s.live[id]
	if !ok {
		return nil, ErrGone
	}
	if !s.now().Before(held.deadline) {
		delete(s.live, id)
		return nil, ErrGone
	}
	if held.state.Owner != owner {
		return nil, ErrGone
	}
	return held, nil
}

// copyState keeps the map and the slice from being shared with the caller. Handing out the stored
// map would let a handler mutate a live scenario without saving it — state that changes on a path
// that was never meant to write is the kind nobody finds later.
func copyState(state State) State {
	state.History = slices.Clone(state.History)
	state.Values = maps.Clone(state.Values)
	if state.Values == nil {
		state.Values = map[string]json.RawMessage{}
	}
	return state
}

func newID() (ID, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return ID(hex.EncodeToString(raw)), nil
}
