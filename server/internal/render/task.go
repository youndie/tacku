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

	// Who is looking, for the rail. Needed here for the same reason it is needed on the board: the
	// navigation names the person at its foot, and a screen without it is one you cannot leave.
	Person domain.MemberID
}

// Screen renders the tree; the caller supplies the schema half through the form builder.
//
// The rail is here because the design puts it here: 3.4 draws the same navigation beside the task
// as beside the board, and a screen you can only leave backwards is a screen you are stuck on. It
// was dropped for one build in exchange for scrolling — the projection that scrolls a screen only
// applies to a column root — and that trade was wrong twice: the rail is what the design asked for,
// and the scroll can be had another way.
//
// The other way is the section list. There is no scroll container in the vocabulary; a
// `paginated_list` inside a bounded box moves its own content (kompot 0.23), so the body is a list
// of blocks rather than a column of them. That is a workaround wearing a component's name, and it
// is written down as one (Q-66).
func (t Task) Screen(comment, status Component) Component {
	return Row("screen-task", 0,
		[]Modifier{FillWidth(), FillHeight(), Background(ColorSurface)},
		Navigation(t.Person, LinkBoard),
		Rule("task-nav-rule", RuleDp, ColorDivider, false),
		Column("screen-task-body", 24,
			[]Modifier{Weight(1), FillHeight(), Padding(32), Background(ColorSurface)},
			BackLink(LinkBoard, string(t.Task.Board)),
			Column("task-heading", 6, nil,
				Text("task-title", t.Task.Title, TextDisplay),
				Text("task-meta", TaskMeta(t.Task), TextMeta),
			),
			PaginatedList("task-sections",
				[]Component{
					Row("task-body", 32, []Modifier{FillWidth()},
						t.left(comment),
						t.sidebar(status),
					),
				},
				"",
				Text("task-sections-empty", "", TextBody),
				FillWidth(), Weight(1)),
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
