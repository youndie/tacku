package render

import (
	"github.com/youndie/tacku/server/internal/domain"
)

// BulkMove is the selection screen: tick several tasks, choose one status, apply once.
//
// It exists as a screen of its own rather than as a mode of the board, and that is the vocabulary
// deciding rather than a preference. There is no hover state and no client-side mode: what a
// selection mode would toggle, this protocol expresses by sending different nodes. A mode is
// therefore a screen — and keeping it apart is what lets the board stay a `screen` with no input on
// it, which is the whole reason it can be delivered conditionally.
//
// It is also the one place a task can move backwards, into Blocked, or across two columns: a card's
// button carries the next status only, and this list carries the full selector.
type BulkMove struct {
	// Tasks are the ones shown, in the order shown.
	Tasks []domain.Task

	// LastBy is who last touched each task. A list of tasks that did not say which of them an agent
	// had touched would be the one list in the product that quietly drops the promise.
	LastBy map[domain.TaskID]domain.Provenance

	// Boxes are the checkboxes, built by the form builder so that every one of them is declared in
	// the schema of the same envelope. Passed in rather than built here for that reason: an input
	// this package invented would name a field nobody declared.
	Boxes map[domain.TaskID]Component

	// Hidden is how many tasks did not fit and why there is a limit at all: every field a form draws
	// is declared in the envelope that carries it, and a continuation arrives as a page, which
	// carries no schema (Q-34). So the list cannot be paged, and a bounded list that says it is
	// bounded beats one that stops without saying so.
	Hidden int
}

func (b BulkMove) Screen(status Component) Component {
	if len(b.Tasks) == 0 {
		return Column("screen-bulk-move", 24,
			[]Modifier{FillWidth(), Padding(32), Background(ColorSurface)},
			b.heading(),
			emptySelection(),
			Spacer("bulk-move-tail"),
		)
	}

	rows := make([]Component, 0, len(b.Tasks))
	for _, task := range b.Tasks {
		rows = append(rows, b.row(task))
	}
	if b.Hidden > 0 {
		rows = append(rows, Text("bulk-move-hidden", b.hiddenLine(), TextMeta))
	}

	return Column("screen-bulk-move", 20,
		[]Modifier{FillWidth(), Padding(32), Background(ColorSurface)},
		b.heading(),
		status,
		// No weight: this is a child of a column root, where the client lays children out as
		// separate items so the screen can scroll, and a weight among them divides nothing. The
		// list takes the height of its rows, which is what a scrolling screen wants anyway.
		Column("bulk-move-list", 8, []Modifier{FillWidth()}, rows...),
		Row("bulk-move-actions", 0, nil,
			PrimaryButton("bulk-move-apply", "Move selected", SubmitForm(BulkFormID),
				PaddingXY(14, 24)),
			Spacer("bulk-move-actions-spacer"),
		),
	)
}

// BulkFormID is the identifier of this form, and it is spelled like a path on purpose.
//
// Nothing on the wire ties a form identifier to the address its submit goes to: `submit_form`
// carries the identifier alone, and the navigation graph maps deeplinks to endpoints that are read
// rather than written. The mapping therefore lives in the client, and the only way for a server to
// add a form without a client release is to make the address derivable — so the route is
// `/submit/` followed by this string, exactly. Recorded as Q-33.
const BulkFormID = "bulk-move"

func (b BulkMove) heading() Component {
	return Column("bulk-move-heading", 6, nil,
		Text("bulk-move-title", "Move several tasks", TextDisplay),
		Text("bulk-move-hint",
			"Tick the tasks that move together, choose where they go, then apply once.",
			TextBodyMuted),
	)
}

func (b BulkMove) hiddenLine() string { return HiddenFromSelection(b.Hidden) }

// row is a checkbox with the provenance stripe and the line of detail the label cannot hold:
// `checkbox_input` has a label and nothing else, so everything else is a sibling.
func (b BulkMove) row(task domain.Task) Component {
	id := "bulk-row-" + string(task.ID)

	stripe, metaStyle := mark(b.LastBy[task.ID])
	line := RowMeta(task, b.LastBy[task.ID])

	return Spaced(id, Marked(id, stripe,
		Column(id+"-body", 4,
			[]Modifier{FillWidth(), Padding(12), Background(ColorSurfaceField)},
			b.Boxes[task.ID],
			Text(id+"-meta", line, metaStyle),
		),
	))
}

// emptySelection is a whole screen of emptiness, so it gets a heading and a way out — the same rule
// the empty board and the empty list follow, and the opposite of the one line an empty column gets.
func emptySelection() Component {
	return Column("bulk-move-empty", 8,
		[]Modifier{Padding(32), Background(ColorSurfaceBlock)},
		Text("bulk-move-empty-title", "There is nothing to move", TextTitle),
		Text("bulk-move-empty-body",
			"Tasks show up here as soon as there are any. Nothing has to be selected first.",
			TextBodyMuted),
		Row("bulk-move-empty-actions", 0, nil,
			Button("bulk-move-empty-go", "Go to the board", Navigate(LinkBoard),
				PaddingXY(12, 20)),
			Spacer("bulk-move-empty-spacer"),
		),
	)
}
