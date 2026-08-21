package render

import (
	"fmt"

	"github.com/youndie/tacku/server/internal/domain"
)

// Task is the screen of one task: what it says, what happened to it, and what can be done.
//
// The one screen addressed by an identifier, which is why it is absent from the navigation graph:
// `endpoint` there is a literal path and the graph has no parameters, so a screen reached by naming
// something is always one the client builds for itself (§12.1).
type Task struct {
	Task     domain.Task
	History  []domain.Change
	Comments []domain.Comment
	Statuses []SelectOption
}

// Screen renders the tree; the caller supplies the schema half through the form builder.
func (t Task) Screen(comment, status Component) Component {
	return Column("screen-task", 24,
		[]Modifier{Padding(32), Background(ColorSurface)},
		Row("task-back-row", 0, nil,
			Button("task-back", "← "+string(t.Task.Board), Navigate(LinkBoard), PaddingXY(12, 20)),
			Spacer("task-back-spacer"),
		),
		Column("task-heading", 6, nil,
			Text("task-title", t.Task.Title, TextDisplay),
			Text("task-meta", t.meta(), TextMeta),
		),
		Row("task-body", 32, []Modifier{Weight(1)},
			t.left(comment),
			t.sidebar(status),
		),
	)
}

func (t Task) meta() string {
	return fmt.Sprintf("%s · created %s", t.Task.ID, day(t.Task.CreatedAt.Format(domain.DuePattern)))
}

func (t Task) left(comment Component) Component {
	children := []Component{
		Column("task-description", 8, nil,
			Text("task-description-label", "DESCRIPTION", TextSubtitle),
			Text("task-description-body", t.description(), TextBody),
		),
		Column("task-activity", 8, nil,
			Text("task-activity-label", "ACTIVITY", TextSubtitle),
			Column("task-activity-list", 8, nil, t.activity()...),
		),
		comment,
		Spacer("task-left-tail"),
	}
	return Column("task-left", 24, []Modifier{Weight(1)}, children...)
}

func (t Task) description() string {
	if t.Task.Body == "" {
		// Written rather than left blank: an empty area says nothing about whether a description is
		// missing or merely short.
		return "No description yet."
	}
	return t.Task.Body
}

// activity is where HistoryLine finally gets called.
//
// It was written with the feed's phrasings — the half that does not name the task, because on this
// screen the reader is already looking at it — and until now nothing used it. A function tested and
// never called is a function whose caller has not been checked, and the test only proved the string.
func (t Task) activity() []Component {
	entries := make([]Component, 0, len(t.History))
	for _, change := range t.History {
		id := fmt.Sprintf("history-%d", change.Seq)

		stripe := ColorDivider
		authorStyle := TextMeta
		if change.By.ByAgent() {
			stripe = ColorAgent
			authorStyle = TextMetaAgent
		}

		entries = append(entries, Row(id, 0, nil,
			Column(id+"-stripe", 0, []Modifier{WidthDp(StripeDp), Background(stripe)}),
			Column(id+"-body", 6,
				[]Modifier{Weight(1), Padding(14), Background(ColorSurfaceBlock)},
				Text(id+"-what", HistoryLine(change), TextBody),
				Text(id+"-who", Author(change), authorStyle),
			),
		))
	}
	if len(entries) == 0 {
		entries = append(entries, Text("history-empty", "Nothing has happened yet.", TextBodyMuted))
	}
	return entries
}

// sidebar carries the facts and the actions.
//
// The full status selector lives here rather than on a board card, which is the other half of the
// decision that put a single-step button there: back, into Blocked and across two columns are
// reachable from this screen and from nowhere else.
func (t Task) sidebar(status Component) Component {
	statusBackground := ColorSurfaceField
	switch t.Task.Status {
	case domain.StatusInProgress, domain.StatusInReview:
		statusBackground = ColorStatusActive
	case domain.StatusDone:
		statusBackground = ColorStatusDone
	case domain.StatusBlocked:
		statusBackground = ColorDanger
	}

	assignee := string(t.Task.Assignee)
	if assignee == "" {
		assignee = "Unassigned"
	}

	due := "No due date"
	if t.Task.Due != "" {
		due = day(t.Task.Due)
	}

	return Column("task-sidebar", 12,
		[]Modifier{WidthDp(320)},
		field("task-status", "Status", StatusName(string(t.Task.Status)), "", statusBackground),
		field("task-assignee", "Assignee", assignee, "", ColorSurfaceField),
		field("task-due", "Due", due, "", ColorSurfaceField),
		field("task-board", "Board", string(t.Task.Board), "", ColorSurfaceField),
		status,
		Spacer("task-sidebar-tail"),
	)
}

func field(id, label, value, helper, background string) Component {
	// Padding then background: the fill takes the padded box. The other order would paint the text
	// and leave the padding outside it, which is §5.1 in one line and the reason the chain is
	// written out rather than assembled somewhere convenient.
	return ReadOnlyField(id, label, value, helper, Padding(12), Background(background))
}
