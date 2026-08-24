package render

import (
	"fmt"
	"strings"
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

	// Failure is what stopped the last reading, or nil if nothing did. A field rather than an
	// inference from the timestamp: how old is too old is a judgement, and whether the last attempt
	// failed is a fact — and which failure it was is the only part a person can act on.
	Failure error
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

	if d.Failure == nil {
		return heading
	}
	return Column("docs-header", 12, nil,
		heading,
		Row("docs-stale", 0, []Modifier{FillWidth(), Padding(12), Background(ColorSurfaceBlock)},
			Text("docs-stale-line", DocsStale(d.Snapshot.TakenAt, d.Now, DocsWhyUnread(d.Failure)), TextNotice)),
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
		if items := docsboard.Column(d.Snapshot.Items, stage); len(items) > 0 {
			out = append(out, d.column(stage, items))
		}
	}
	return out
}

func (d DocsBoard) column(stage string, items []docsboard.Item) Component {
	id := "docs-column-" + stage
	if stage == docsboard.NoStage {
		id = "docs-column-none"
	}

	cards := make([]Component, 0, len(items))
	for _, item := range items {
		cards = append(cards, d.card(item))
	}

	body := []Component{
		Row(id+"-head", 0, nil,
			Text(id+"-name", DocsStageName(stage), TextSubtitle),
			Spacer(id+"-head-spacer"),
			Text(id+"-count", fmt.Sprint(len(cards)), TextMeta),
		),
	}
	// Above the cards and not under them. Under them it was invisible: a list takes the height that
	// is left, so anything after it starts below the bottom of the screen — which the tree does not
	// say and only a picture shows.
	if done := docsboard.DoneCount(d.Snapshot.Items, stage); done > 0 {
		body = append(body, Text(id+"-done", DocsDone(done), TextMeta))
	}
	body = append(body, PaginatedList(id+"-list", cards, "", nil, FillWidth()))

	return Column(id, 12,
		[]Modifier{Weight(1), Padding(12), Background(ColorSurfaceBlock)},
		body...)
}

// card is the item, and it opens the item's own screen.
//
// Not the file it stands for: the vocabulary has no action that leaves the application (Q-72), so
// the text is brought here instead of the reader being sent there. Which is not only a
// consolation — the file is in a repository the reader may not have checked out.
func (d DocsBoard) card(item docsboard.Item) Component {
	id := "docs-card-" + item.ID

	body := []Component{
		Text(id+"-meta", DocsCardMeta(item), TextMeta),
		Text(id+"-title", item.Title, TextBody),
	}
	if len(item.BlockedBy) > 0 {
		body = append(body, Text(id+"-blocked", DocsBlockedBy(item.BlockedBy), TextMeta))
	}

	return Spaced(id, Opens(
		Column(id+"-body", 6,
			[]Modifier{FillWidth(), Padding(12), Background(ColorSurfaceField)},
			body...),
		Navigate(LinkDocsItem+item.ID),
	))
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
func UnreadableDocsBoard(person domain.MemberID, failure error, repo, ref, root string) Component {
	return Row("screen-docs", 0,
		[]Modifier{FillWidth(), FillHeight(), Background(ColorSurface)},
		Navigation(person, LinkDocsBoard),
		Rule("docs-nav-rule", RuleDp, ColorDivider, false),
		Column("docs", 24,
			[]Modifier{Weight(1), Padding(32)},
			Column("docs-unreadable", 8,
				[]Modifier{Padding(32), Background(ColorSurfaceBlock)},
				Text("docs-unreadable-title", "The backlog could not be read", TextTitle),
				Text("docs-unreadable-why", DocsWhyUnread(failure), TextBody),
				Text("docs-unreadable-source", DocsSource(repo, ref, root), TextMeta),
				Text("docs-unreadable-body",
					"Nothing has been read from the source since this server started, so there is not even an old reading to show.",
					TextBodyMuted),
			),
			Spacer("docs-unreadable-tail"),
		),
	)
}

// DocsItem is one item of that backlog, read inside the application.
//
// It exists because the alternative does not: the vocabulary has no action that leaves the
// application (Q-72), so a card could name an item and nothing more, and the board was a wall of
// dead ends. The text is the source's, rendered rather than linked to.
type DocsItem struct {
	Person domain.MemberID
	Item   docsboard.Item
}

func (d DocsItem) Screen() Component {
	body := []Component{
		BackLink(LinkDocsBoard, RouteTitle(LinkDocsBoard)),
		Column("docs-item-heading", 6, nil,
			Text("docs-item-title", d.Item.Title, TextDisplay),
			Text("docs-item-meta", DocsCardMeta(d.Item), TextMeta),
		),
	}
	if len(d.Item.BlockedBy) > 0 {
		body = append(body, Text("docs-item-blocked", DocsBlockedBy(d.Item.BlockedBy), TextBodyMuted))
	}
	body = append(body, prose("docs-item-body", d.Item.Body, d.Item.ID)...)
	body = append(body, Text("docs-item-path", d.Item.Path, TextMeta))

	return Row("screen-docs-item", 0,
		[]Modifier{FillWidth(), FillHeight(), Background(ColorSurface)},
		Navigation(d.Person, LinkDocsBoard),
		Rule("docs-item-nav-rule", RuleDp, ColorDivider, false),
		// The padding is inside the list rather than on it: a list insets its viewport, so padding
		// put here clips the scroll at both ends instead of framing it.
		PaginatedList("docs-item-scroll", []Component{
			Column("docs-item", 16, []Modifier{FillWidth(), Padding(32)}, body...),
		}, "", nil, Weight(1)),
	)
}

// prose turns the source's markup into what the vocabulary has: lines of text carrying a token.
//
// Deliberately shallow. A heading becomes a heading, a list item becomes a line with one marker, a
// paragraph becomes a paragraph, and the markers inside a line — emphasis, code, links — are left
// as the author typed them. Anything more would be a markdown renderer built out of `text`, and the
// first thing it could not do is the thing it exists for: a link inside a paragraph is not
// pressable in this vocabulary at all, so a renderer that dressed one up would be lying about it.
//
// Shallow is not the same as careless, and two rules here were written by looking at a real item
// rather than at the fixture. A table came out as one long line, because joining a paragraph is
// what happens to any run of non-blank lines and a table is exactly that — so a row keeps its own
// line, and so does everything inside a fence. And the file's own first heading repeats the title
// that is already at the top of the screen, since the method's template opens every item with
// `# B-NN — <title>`: it is dropped, by matching the identifier rather than the words.
func prose(id, body, item string) []Component {
	var out []Component
	var paragraph []string
	fenced := false
	dropped := false

	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		out = append(out, Text(fmt.Sprintf("%s-%d", id, len(out)),
			strings.Join(paragraph, " "), TextBody))
		paragraph = nil
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		node := fmt.Sprintf("%s-%d", id, len(out))

		switch {
		case strings.HasPrefix(trimmed, "```"):
			flush()
			fenced = !fenced
			out = append(out, Text(node, trimmed, TextBody))
		case fenced, strings.HasPrefix(trimmed, "|"):
			// A line that is part of a structure: kept as a line, because joining it into a
			// paragraph is what turned a table into one unreadable sentence.
			flush()
			out = append(out, Text(node, line, TextBody))
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "#"):
			flush()
			heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if !dropped && strings.HasPrefix(heading, item) {
				dropped = true
				continue
			}
			out = append(out, Text(node, heading, TextSubtitle))
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "), strings.HasPrefix(trimmed, "+ "):
			flush()
			out = append(out, Text(node, DocsBullet(trimmed[2:]), TextBody))
		default:
			paragraph = append(paragraph, trimmed)
		}
	}
	flush()
	return out
}
