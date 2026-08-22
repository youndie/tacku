package render_test

import (
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/render"
)

// The two lines under the panel's values say what happened, and say it in the largest unit that
// still means something.
//
// They existed as empty strings for the life of the project: `read_only_field` carries a helper and
// every one of them was passed "". Nothing was broken and nothing was missing from the vocabulary —
// the text was never written, which is a kind of absence no check notices.
func TestTheLineUnderTheStatusNamesWhoAndWhen(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	agent := domain.Agent("anna-agent", "dev", "anna")
	person := domain.Human("ivan")

	cases := []struct {
		name    string
		history []domain.Change
		want    string
	}{
		{
			name:    "an agent five hours ago",
			history: []domain.Change{{Kind: domain.ChangeStatusMoved, By: agent, CreatedAt: now.Add(-5 * time.Hour)}},
			want:    "Changed by your agent 5 hours ago",
		},
		{
			// The singular is a rule of this product, not a nicety: one place decides it.
			name:    "a person an hour ago",
			history: []domain.Change{{Kind: domain.ChangeStatusMoved, By: person, CreatedAt: now.Add(-time.Hour)}},
			want:    "Changed by ivan 1 hour ago",
		},
		{
			name:    "creation counts, because a task starts somewhere",
			history: []domain.Change{{Kind: domain.ChangeTaskCreated, By: person, CreatedAt: now.Add(-3 * 24 * time.Hour)}},
			want:    "Changed by ivan 3 days ago",
		},
		{
			// A comment does not move a status, and the line is about the status.
			name: "the latest status change, not the latest change",
			history: []domain.Change{
				{Kind: domain.ChangeStatusMoved, By: agent, CreatedAt: now.Add(-2 * time.Hour)},
				{Kind: domain.ChangeCommentPosted, By: person, CreatedAt: now.Add(-time.Minute)},
			},
			want: "Changed by your agent 2 hours ago",
		},
		{
			// Nothing known is said as nothing, not as a guess with a name in it.
			name:    "no history at all",
			history: nil,
			want:    "",
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := render.StatusHelper(one.history, now); got != one.want {
				t.Errorf("got %q, want %q", got, one.want)
			}
		})
	}
}

// How far away a due date is, counted in whole days rather than in hours.
//
// Midnight to midnight, so "tomorrow" is the next calendar day whatever time it is now: a task due
// tomorrow morning is due tomorrow at 23:00 today, and "in 0 days" would be a lie told by division.
func TestTheLineUnderTheDueDateCountsWholeDays(t *testing.T) {
	now := time.Date(2026, 8, 23, 23, 0, 0, 0, time.UTC)

	cases := map[string]string{
		"2026-08-23": "Today",
		"2026-08-24": "Tomorrow",
		"2026-08-22": "Yesterday",
		"2026-08-30": "In 7 days",
		"2026-08-25": "In 2 days",
		"2026-08-20": "3 days ago",
		"":           "",
		"not a date": "",
	}

	for iso, want := range cases {
		if got := render.DueHelper(iso, now); got != want {
			t.Errorf("%q: got %q, want %q", iso, got, want)
		}
	}
}
