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
//
// A column at the root, and nothing among its children carries a weight — the two go together. The
// client lays a column root's children out as separate items so that a screen taller than the window
// can be scrolled, which is what this screen needs: a description, a history and a comment box add
// up past the fold on any window worth using. The price is that a `weight` among those children
// divides nothing, so the body takes the height of its content instead of the height of the screen.
//
// It was the other way round for a few hours — a row at the root so that `task-body` could be
// weighted — and the result was a task you could not scroll. Height that fills the window is worth
// less here than reaching the button at the bottom.
func (t Task) Screen(comment, status Component) Component {
	return Column("screen-task", 24,
		[]Modifier{FillWidth(), Padding(32), Background(ColorSurface)},
		Column("task-heading", 6, nil,
			Text("task-title", t.Task.Title, TextDisplay),
			Text("task-meta", TaskMeta(t.Task), TextMeta),
		),
		Row("task-body", 32, []Modifier{FillWidth()},
			t.left(comment),
			t.sidebar(status),
		),
	)
}

func (t Task) left(comment Component) Component {
	children := []Component{
		Column("task-description", 8, nil,
			Text("task-description-label", "DESCRIPTION", TextSubtitle),
			Text("task-description-body", DescriptionValue(t.Task.Body), TextBody),
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

		entries = append(entries, Marked(id, stripe,
			Column(id+"-body", 6,
				[]Modifier{FillWidth(), Padding(14), Background(ColorSurfaceBlock)},
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

	return Column("task-sidebar", 12,
		[]Modifier{WidthDp(320)},
		// The labels are this screen's; the values are copy.go's, including the stand-ins it shows
		// where there is nothing to render — a card and this sidebar used to spell one of them two
		// different ways.
		field("task-status", "Status", StatusName(string(t.Task.Status)), "", statusBackground),
		field("task-assignee", "Assignee", AssigneeValue(t.Task.Assignee), "", ColorSurfaceField),
		field("task-due", "Due", DueValue(t.Task.Due), "", ColorSurfaceField),
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
