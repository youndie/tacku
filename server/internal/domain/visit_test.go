package domain_test

import (
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

// The rule of B-27 in one place, checked as a rule rather than through a server.
//
// The four cases below are the four things the boundary has to do, and each one is a different way
// of being wrong: replaying the journal at somebody who never left, emptying the screen being read,
// never advancing at all, and advancing over changes nobody was ever shown.
func TestArrive(t *testing.T) {
	const gap = 8 * time.Hour
	base := time.Date(2026, 8, 20, 18, 40, 0, 0, time.UTC)

	t.Run("a first visit reads everything and advances nothing", func(t *testing.T) {
		got := domain.Visit{Boundary: domain.Start, Pending: domain.Start}.
			Arrive(base, domain.CursorAt(5), gap)

		if got.Boundary != domain.Start {
			t.Errorf("the first visit is measured from %q, not the start of the journal", got.Boundary)
		}
		if got.Pending != domain.CursorAt(5) {
			t.Errorf("the next visit would start at %q, want c5", got.Pending)
		}
		if got.Away != 0 {
			t.Errorf("somebody who has never been here was away for %s", got.Away)
		}
	})

	t.Run("a reload inside a visit leaves the boundary alone", func(t *testing.T) {
		before := domain.Visit{Boundary: domain.Start, Pending: domain.CursorAt(5), At: base}

		got := before.Arrive(base.Add(time.Minute), domain.CursorAt(7), gap)

		if got.Boundary != domain.Start {
			t.Errorf("a reload a minute later moved the boundary to %q; the screen being read would empty", got.Boundary)
		}
		if got.Pending != domain.CursorAt(7) {
			t.Errorf("the next visit would start at %q, want c7 — what arrived during this visit was shown", got.Pending)
		}
		if got.Away != 0 {
			t.Errorf("a reload claimed an absence of %s", got.Away)
		}
	})

	t.Run("coming back after the gap starts from where the last visit ended", func(t *testing.T) {
		before := domain.Visit{Boundary: domain.Start, Pending: domain.CursorAt(7), At: base}

		got := before.Arrive(base.Add(9*time.Hour), domain.CursorAt(9), gap)

		if got.Boundary != domain.CursorAt(7) {
			t.Errorf("the new visit is measured from %q, want c7 — the end of the previous one", got.Boundary)
		}
		if got.Pending != domain.CursorAt(9) {
			t.Errorf("the next visit would start at %q, want c9", got.Pending)
		}
		if got.Away != 9*time.Hour {
			t.Errorf("the absence came out as %s, want 9h", got.Away)
		}
	})

	t.Run("the gap is a floor and not a ceiling", func(t *testing.T) {
		before := domain.Visit{Boundary: domain.Start, Pending: domain.CursorAt(7), At: base}

		short := before.Arrive(base.Add(gap-time.Second), domain.CursorAt(9), gap)
		if short.Boundary != domain.Start {
			t.Errorf("a second short of the gap counted as leaving")
		}
		exact := before.Arrive(base.Add(gap), domain.CursorAt(9), gap)
		if exact.Boundary != domain.CursorAt(7) {
			t.Errorf("exactly the gap did not count as leaving")
		}
	})

	// A clock that steps backwards — a correction, a machine that never had the right time — must
	// not read as an absence. The safe answer is the one that shows more rather than less.
	t.Run("a clock that went backwards is not an absence", func(t *testing.T) {
		before := domain.Visit{Boundary: domain.Start, Pending: domain.CursorAt(7), At: base}

		got := before.Arrive(base.Add(-9*time.Hour), domain.CursorAt(9), gap)
		if got.Boundary != domain.Start {
			t.Errorf("time going backwards moved the boundary to %q", got.Boundary)
		}
	})
}
