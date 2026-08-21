package render

import (
	"fmt"

	"github.com/youndie/tacku/server/internal/domain"
)

// Board builds the board screen.
//
// It carries no input and is therefore a `screen` rather than a `form`, which is why it can be
// cached — and it can carry no input because a card's button is a `perform`, the action that acts on
// one item of a list. Before that existed the only way to move a task from here was a per-card
// input, and the busiest screen of the product was the one that could not be cached.
type Board struct {
	Title   string
	MoveURL string
	Tasks   []domain.Task

	// Who last touched each task, so that a card can carry provenance.
	//
	// It was missing until a check went looking for the agent colour and found none: every card
	// drew a grey stripe regardless, so the board — the screen a team looks at most — was the one
	// place the product's central promise was not kept. Nothing failed; the signal was simply
	// absent, which is how a promise stops being kept without anybody deciding to stop keeping it.
	LastBy map[domain.TaskID]domain.Provenance

	Changed int
}

// next is the status a card's button moves to.
//
// The chain is linear, and that is the price the design named out loud: back, into Blocked and
// across two columns are not reachable from a card. A status with nowhere to go returns false, and
// then the card carries no button at all — shown by its absence rather than stated in a caption.
func next(status domain.Status) (domain.Status, bool) {
	switch status {
	case domain.StatusTodo:
		return domain.StatusInProgress, true
	case domain.StatusInProgress:
		return domain.StatusInReview, true
	case domain.StatusInReview:
		return domain.StatusDone, true
	}
	return "", false
}

func (b Board) Screen() Component {
	columns := make([]Component, 0, len(domain.Statuses))
	for _, status := range domain.Statuses {
		columns = append(columns, b.column(status))
	}

	return Row("screen-board", 0,
		[]Modifier{Background(ColorSurface)},
		Column("board-nav-placeholder", 0, []Modifier{WidthDp(240), Background(ColorSurfaceBlock)}),
		Rule("board-nav-rule", RuleDp, ColorDivider, false),
		Column("board", 20,
			[]Modifier{Weight(1), Padding(32)},
			b.header(),
			Row("board-columns", 12, []Modifier{Weight(1)}, columns...),
		),
	)
}

func (b Board) header() Component {
	return Row("board-header", 0, nil,
		Column("board-heading", 6, nil,
			Text("board-title", b.Title, TextDisplay),
			Text("board-count", b.summary(), TextBodyMuted),
		),
		Spacer("board-header-spacer"),
		Button("board-new", "New task", Navigate(LinkNewTask),
			PaddingXY(12, 20), Background(ColorAccent)),
	)
}

func (b Board) summary() string {
	open := 0
	for _, task := range b.Tasks {
		if task.Status != domain.StatusDone {
			open++
		}
	}
	word := "tasks"
	if open == 1 {
		word = "task"
	}
	return fmt.Sprintf("%d open %s · %d changed since your last visit", open, word, b.Changed)
}

func (b Board) column(status domain.Status) Component {
	id := "column-" + string(status)

	cards := make([]Component, 0)
	for _, task := range b.Tasks {
		if task.Status == status {
			cards = append(cards, b.card(task))
		}
	}

	return Column(id, 12,
		[]Modifier{Weight(1), Padding(12), Background(ColorSurfaceBlock)},
		Row(id+"-head", 0, nil,
			Text(id+"-name", upper(StatusName(string(status))), TextSubtitle),
			Spacer(id+"-head-spacer"),
			Text(id+"-count", fmt.Sprint(len(cards)), TextMeta),
		),
		PaginatedList(id+"-list", cards, "", emptyColumn(id, status)),
		Spacer(id+"-tail"),
	)
}

func (b Board) card(task domain.Task) Component {
	id := "card-" + string(task.ID)

	stripe, metaStyle, meta := mark(b.LastBy[task.ID], cardMeta(task))

	body := []Component{
		Text(id+"-title", task.Title, TextBody),
		Text(id+"-meta", meta, metaStyle),
	}

	if target, ok := next(task.Status); ok {
		body = append(body, Button(id+"-move", "Move to "+StatusName(string(target)),
			// Both values are text: the value hierarchy does not degrade, so a shape the client does
			// not know costs the whole screen rather than this button.
			Perform(b.MoveURL, map[string]any{
				"task":   FieldText(string(task.ID)),
				"status": FieldText(string(target)),
			})))
	}

	return Row(id, 0, nil,
		Column(id+"-stripe", 0, []Modifier{WidthDp(StripeDp), Background(stripe)}),
		Column(id+"-body", 8,
			[]Modifier{Weight(1), Padding(12), Background(ColorSurfaceField)},
			body...),
	)
}

func cardMeta(task domain.Task) string {
	meta := string(task.ID)
	if task.Assignee != "" {
		meta += " · " + string(task.Assignee)
	} else {
		meta += " · unassigned"
	}
	if task.Due != "" {
		meta += " · due " + day(task.Due)
	}
	return meta
}

// emptyColumn is one line and no call to action. Four columns empty on a Monday morning would be
// four invitations on one screen, and the heading already says the column is empty — the line says
// why that is fine.
func emptyColumn(id string, status domain.Status) Component {
	text := "Nothing here yet."
	if status == domain.StatusDone {
		text = "Nothing finished yet this sprint."
	}
	return Text(id+"-empty", text, TextBodyMuted)
}

func upper(value string) string {
	out := make([]rune, 0, len(value))
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		out = append(out, r)
	}
	return string(out)
}

// EmptyWorkspace is the board screen before there is a board.
//
// A full-screen emptiness earns a heading, an explanation and a way out — the same construction the
// feed uses when nothing has happened. A column that is merely empty gets one line; the scale of
// the emptiness decides the form.
func EmptyWorkspace() Component {
	return Row("screen-board", 0,
		[]Modifier{Background(ColorSurface)},
		Column("board-nav-placeholder", 0, []Modifier{WidthDp(240), Background(ColorSurfaceBlock)}),
		Rule("board-nav-rule", RuleDp, ColorDivider, false),
		Column("board", 24,
			[]Modifier{Weight(1), Padding(32)},
			Column("board-empty", 8,
				[]Modifier{Padding(32), Background(ColorSurfaceBlock)},
				Text("board-empty-title", "No boards yet", TextTitle),
				Text("board-empty-body",
					"Boards are created by people. Your agent can fill one with tasks, but it cannot make one.",
					TextBodyMuted),
				Row("board-empty-actions", 0, nil,
					Button("board-empty-new", "New board", Navigate(LinkNewBoard),
						PaddingXY(12, 20), Background(ColorAccent)),
					Spacer("board-empty-spacer"),
				),
			),
			Spacer("board-empty-tail"),
		),
	)
}

// TaskRow is one line of a task list: the same card as on a board, without the move button. A list
// filtered by status has no next status to name.
// mark is the one place a list decides how a person and an agent look different.
//
// Shared by every list in the product on purpose. It used to be written out inside the board card
// and nowhere else, and the row of a filtered list — same product, same promise — drew a grey
// stripe for everyone. One copy means the next list cannot forget.
func mark(by domain.Provenance, human string) (stripe, style, meta string) {
	if by.ByAgent() {
		return ColorAgent, TextMetaAgent, "Agent · on behalf of " + string(by.OnBehalfOf)
	}
	return ColorDivider, TextMeta, human
}

// TaskRow is one line of a filtered list — "my tasks" and every page after it.
func TaskRow(task domain.Task, by domain.Provenance) Component {
	id := "row-" + string(task.ID)
	stripe, metaStyle, meta := mark(by, cardMeta(task))
	// The status stays on the line either way: this list crosses columns, so a row that does not
	// say where it stands is a row the reader has to open to place.
	meta += " · " + StatusName(string(task.Status))
	return Row(id, 0, nil,
		Column(id+"-stripe", 0, []Modifier{WidthDp(StripeDp), Background(stripe)}),
		Column(id+"-body", 6,
			[]Modifier{Weight(1), Padding(12), Background(ColorSurfaceField)},
			Text(id+"-title", task.Title, TextBody),
			Text(id+"-meta", meta, metaStyle),
		),
	)
}

// EmptyMyTasks is a whole screen of emptiness, so it gets a heading and a way out — unlike an empty
// column, which gets one line. The scale of the emptiness decides the form.
func EmptyMyTasks() Component {
	return Column("my-tasks-empty", 8,
		[]Modifier{Padding(32), Background(ColorSurfaceBlock)},
		Text("my-tasks-empty-title", "Nothing is assigned to you", TextTitle),
		Text("my-tasks-empty-body",
			"Tasks show up here when someone assigns them to you — or when your agent picks work up on your behalf.",
			TextBodyMuted),
		Row("my-tasks-empty-actions", 0, nil,
			Button("my-tasks-empty-go", "Go to the board", Navigate(LinkBoard),
				PaddingXY(12, 20)),
			Spacer("my-tasks-empty-spacer"),
		),
	)
}
