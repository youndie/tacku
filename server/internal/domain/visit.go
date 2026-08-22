package domain

import "time"

// Visit is where one person had read up to, and how that boundary moves without being asked.
//
// Four values rather than one, and each of them is there because "since your last visit" is a
// promise about a moment that has already passed:
//
//   - Boundary is what the feed is measured from. It never moves at the moment the feed is
//     rendered: advancing it on arrival would empty the very screen the person came to read, and
//     would also make two identical requests produce two different bodies — which SPEC.md §16.2
//     forbids outright, because a screen whose body changes per request can never answer 304.
//   - Pending is where the next visit will start. It is the end of the journal as of the last time
//     this person was seen, so the boundary only ever moves over entries a visit was offered.
//   - At is when this person was last seen. It slides with every arrival rather than being fixed at
//     the start of a visit: a fixed window would begin a new visit under somebody who never left,
//     and the feed they were reading would empty in front of them.
//   - Away is how long this person had been gone when the current visit began. Kept rather than
//     computed at render time so that the sentence in the headline stays the same for the whole
//     visit; a phrase recomputed against the clock would change the body every hour and would cost
//     the 304 for nothing.
type Visit struct {
	Boundary Cursor
	Pending  Cursor
	At       time.Time
	Away     time.Duration
}

// DefaultVisitGap is how long away has to be before coming back counts as a new visit.
//
// NOT A MEASUREMENT. Nobody has used this product yet, so the distribution of gaps between two
// arrivals of the same person does not exist to be measured, and a number invented here would look
// exactly like one that had been. What is chosen instead is a direction and a scenario:
//
//   - The direction. Too long a gap shows somebody the same feed twice, which costs a glance. Too
//     short a gap moves the boundary over changes nobody read, and those are gone from the feed for
//     good. The two mistakes are not the same size, so the value sits at the long end.
//   - The scenario. The shortest absence this product names for itself is a night — the catch-up
//     screen exists for the person whose agent worked while they slept. Eight hours is that
//     absence, and it is longer than any interruption inside a working day.
//
// The number that would replace it is the gap after which a person expects the feed to have reset,
// and measuring it needs people. Until then this is configuration, not a constant of the domain.
const DefaultVisitGap = 8 * time.Hour

// Arrive is this visit as it stands after the person asked for the feed at now, with the journal
// standing at latest.
//
// Pure, and separate from storage, because it is the whole decision of B-27; everything else is
// where the four values are kept.
func (v Visit) Arrive(now time.Time, latest Cursor, gap time.Duration) Visit {
	next := v
	// A person who has never been here is not returning from anywhere, so nothing advances and no
	// sentence about a previous visit is owed. Their boundary stays at the start of the journal,
	// which is the only answer that does not hide what happened before they joined.
	if !v.At.IsZero() && now.Sub(v.At) >= gap {
		next.Boundary = v.Pending
		next.Away = now.Sub(v.At)
	}
	next.Pending = latest
	next.At = now
	return next
}

// Dismiss is the explicit half of the boundary: "mark all as seen", pressed at now with the journal
// standing at latest.
//
// Both cursors and not just the boundary. Leaving Pending where it was would let the next arrival
// set the boundary back to the end of the previous visit, and the feed would come back the morning
// after it had been dismissed — the button undone by walking away.
//
// Away goes back to zero because the sentence it feeds is about a previous visit, and this one is
// happening now.
func Dismiss(now time.Time, latest Cursor) Visit {
	return Visit{Boundary: latest, Pending: latest, At: now}
}
