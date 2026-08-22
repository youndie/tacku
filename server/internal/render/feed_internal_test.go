// Inside the package rather than beside it, which the rest of this package's tests are not.
//
// The sentence about the previous visit is assembled from a duration and never appears anywhere a
// black-box test could vary it: the screen shows whatever the visit recorded, and a test driving
// the server can reach one value per run. The branches — plural, hours against days, the rounding —
// are only reachable from here.
package render

import (
	"testing"
	"time"
)

func TestLastVisit(t *testing.T) {
	cases := []struct {
		away time.Duration
		want string
	}{
		{0, ""},
		{-time.Hour, ""},
		{30 * time.Minute, "you were last here under an hour ago"},
		{time.Hour, "you were last here 1 hour ago"},
		{9 * time.Hour, "you were last here 9 hours ago"},
		// Rounded, not truncated: five minutes short of nine hours is nine hours to a person.
		{8*time.Hour + 55*time.Minute, "you were last here 9 hours ago"},
		{47 * time.Hour, "you were last here 47 hours ago"},
		// Past two days the number of hours stops being an answer anybody reads.
		{48 * time.Hour, "you were last here 2 days ago"},
		{25 * time.Hour * 24, "you were last here 25 days ago"},
	}

	for _, c := range cases {
		if got := lastVisit(c.away); got != c.want {
			t.Errorf("an absence of %s reads as %q, want %q", c.away, got, c.want)
		}
	}
}
