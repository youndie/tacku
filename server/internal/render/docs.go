package render

import (
	"fmt"
	"time"

	"github.com/youndie/tacku/server/internal/docsboard"
	"github.com/youndie/tacku/server/internal/domain"
)

// DocsBoard is the read-only view over a backlog that lives in a repository.
//
// It looks like the product's board and shares none of its code, which is deliberate. A card here
// has no button, because there is nothing this screen may do: the repository owns these items and
// they change in a pull request. Rendering them through Board would have meant a Task with invented
// fields and a move button that had to be suppressed — a screen that has to hide what it inherited
// is one release away from showing it.
type DocsBoard struct {
	Person   domain.MemberID
	Snapshot docsboard.Snapshot
	Now      time.Time

	// Stale is set when the source could not be reached and what is on the screen is the previous
	// reading. It is a field rather than an inference from the timestamp: how old is too old is a
	// judgement, and whether the last attempt failed is a fact.
	Stale bool
}

func (d DocsBoard) Screen() Component {
	return Row("screen-docs", 0,
		[]Modifier{FillWidth(), FillHeight(), Background(ColorSurface)},
		Navigation(d.Person, LinkDocsBoard),
		Rule("docs-nav-rule", RuleDp, ColorDivider, false),
		Column("docs", 20,
			[]Modifier{Weight(1), Padding(32)},
			d.header(),
			Row("docs-columns", 12, []Modifier{Weight(1)}, d.columns()...),
		),
	)
}

func (d DocsBoard) header() Component {
	heading := Column("docs-heading", 6, nil,
		// The source names its own backlog and this screen repeats that name rather than inventing
		// one: what a person recognises is the sentence at the top of the file they work in.
		Text("docs-title", d.Snapshot.Title, TextDisplay),
		Text("docs-count", DocsSummary(d.open(), d.done(), d.Snapshot.TakenAt, d.Now), TextBodyMuted),
	)

	if !d.Stale {
		return heading
	}
	return Column("docs-header", 12, nil,
		heading,
		Row("docs-stale", 0, []Modifier{FillWidth(), Padding(12), Background(ColorSurfaceBlock)},
			Text("docs-stale-line", DocsStale(d.Snapshot.TakenAt, d.Now), TextNotice)),
	)
}

// columns are the stages with work left in them.
//
// A finished stage is left out rather than drawn empty, and the count in the headline is what keeps
// that honest: a repository that has been running for a while has more finished stages than open
// ones, and nine columns of which three have anything in them is a board that has to be read before
// it can be looked at.
func (d DocsBoard) columns() []Component {
	var out []Component
	for _, stage := range d.Snapshot.Stages {
		if items := docsboard.Column(d.Snapshot.Items, stage.ID); len(items) > 0 {
			out = append(out, d.column(stage, items))
		}
	}
	return out
}

func (d DocsBoard) column(stage docsboard.Stage, items []docsboard.Item) Component {
	id := "docs-column-" + stage.ID
	if stage.ID == docsboard.NoStage {
		id = "docs-column-none"
	}

	cards := make([]Component, 0, len(items))
	for _, item := range items {
		cards = append(cards, d.card(item))
	}

	body := []Component{
		Row(id+"-head", 0, nil,
			Text(id+"-name", stage.Title, TextSubtitle),
			Spacer(id+"-head-spacer"),
			Text(id+"-count", fmt.Sprint(len(cards)), TextMeta),
		),
	}
	// Above the cards and not under them. Under them it was invisible: a list takes the height that
	// is left, so anything after it starts below the bottom of the screen — which the tree does not
	// say and only a picture shows.
	if done := docsboard.DoneCount(d.Snapshot.Items, stage.ID); done > 0 {
		body = append(body, Text(id+"-done", DocsDone(done), TextMeta))
	}
	body = append(body, PaginatedList(id+"-list", cards, "", nil, FillWidth()))

	return Column(id, 12,
		[]Modifier{Weight(1), Padding(12), Background(ColorSurfaceBlock)},
		body...)
}

// card is the item, and it opens nothing.
//
// Not an oversight and not laziness: the vocabulary has no action that leaves the application, so a
// card cannot lead to the file it stands for — see the question log. What it can do is name the item
// the way the repository names it, which is what a person searches for in their checkout.
func (d DocsBoard) card(item docsboard.Item) Component {
	id := "docs-card-" + item.ID

	body := []Component{
		Text(id+"-meta", DocsCardMeta(item), TextMeta),
		Text(id+"-title", item.Title, TextBody),
	}
	if len(item.BlockedBy) > 0 {
		body = append(body, Text(id+"-blocked", DocsBlockedBy(item.BlockedBy), TextMeta))
	}

	return Spaced(id, Column(id+"-body", 6,
		[]Modifier{FillWidth(), Padding(12), Background(ColorSurfaceField)},
		body...))
}

func (d DocsBoard) open() int {
	open := 0
	for _, item := range d.Snapshot.Items {
		if !item.Done() {
			open++
		}
	}
	return open
}

func (d DocsBoard) done() int { return len(d.Snapshot.Items) - d.open() }

// UnreadableDocsBoard is the screen when the source has never been read at all.
//
// A screen and not an error status: the client would turn a 5xx into its own message, and the person
// looking at it is the one who can fix this — the repository, the branch or the credential is wrong,
// or the source is down. It says which of those it cannot tell apart rather than pretending to know.
func UnreadableDocsBoard(person domain.MemberID) Component {
	return Row("screen-docs", 0,
		[]Modifier{FillWidth(), FillHeight(), Background(ColorSurface)},
		Navigation(person, LinkDocsBoard),
		Rule("docs-nav-rule", RuleDp, ColorDivider, false),
		Column("docs", 24,
			[]Modifier{Weight(1), Padding(32)},
			Column("docs-unreadable", 8,
				[]Modifier{Padding(32), Background(ColorSurfaceBlock)},
				Text("docs-unreadable-title", "The backlog could not be read", TextTitle),
				Text("docs-unreadable-body",
					"Nothing has been read from the source yet. Either it cannot be reached, or this deployment is pointed at a repository, a branch or a directory that does not hold a backlog.",
					TextBodyMuted),
			),
			Spacer("docs-unreadable-tail"),
		),
	)
}
