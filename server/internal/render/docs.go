package render

import (
	"fmt"
	"regexp"
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
// numbered is a list item the author numbered: `1. `, `2) `.
var numbered = regexp.MustCompile(`^[0-9]+[.)]\s`)

func prose(id, body, item string) []Component {
	var out []Component
	var paragraph, code []string
	var rows []TableRow

	// The line of a list item being built, and the indent it was written at.
	//
	// Buffered rather than emitted where it starts, because a source wraps: an item is written
	// across as many lines as it needs, the ones after the first carrying no marker. Emitted line
	// by line, the second half of every item became a paragraph of its own — and the second halves
	// of neighbouring items were then joined into one. That is what "the markdown does not render"
	// looked like from the outside.
	var listItem []string
	var listAt []Modifier
	fenced, dropped := false, false

	node := func() string { return fmt.Sprintf("%s-%d", id, len(out)) }

	// One flush for every block this understands, because a block ends the same way whatever it is:
	// something that is not it comes next. It closes all of them rather than the first non-empty
	// one: a `switch` here meant that a table written straight under a paragraph, with no blank
	// line between them, flushed the paragraph and dropped every row of the table without a word.
	flush := func() {
		if len(paragraph) > 0 {
			out = append(out, Text(node(), strings.Join(paragraph, " "), TextBody))
			paragraph = nil
		}
		if len(listItem) > 0 {
			out = append(out, Text(node(), strings.Join(listItem, " "), TextBody, listAt...))
			listItem, listAt = nil, nil
		}
		if len(rows) > 0 {
			out = append(out, Table(node(), rows, FillWidth()))
			rows = nil
		}
		if len(code) > 0 {
			lines := make([]Component, 0, len(code))
			for _, line := range code {
				lines = append(lines, Text(fmt.Sprintf("%s-code-%d", node(), len(lines)), line, TextBodyMuted))
			}
			out = append(out, Column(node(), 0,
				[]Modifier{FillWidth(), Padding(12), Background(ColorSurfaceField)}, lines...))
			code = nil
		}
	}

	// A line of a list keeps the depth it was written at. Two spaces to a level, which is what the
	// sources use; a deeper nesting than three is drawn at three rather than marching off the edge.
	indented := func(line string) []Modifier {
		depth := (len(line) - len(strings.TrimLeft(line, " "))) / 2
		if depth > 3 {
			depth = 3
		}
		if depth == 0 {
			return nil
		}
		return []Modifier{PaddingXY(0, 12*depth)}
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)

		if fenced {
			if strings.HasPrefix(trimmed, "```") {
				fenced = false
				flush()
			} else {
				code = append(code, line)
			}
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "```"):
			flush()
			fenced = true
		case strings.HasPrefix(trimmed, "|"):
			if len(rows) == 0 {
				flush()
			}
			if row, ok := tableRow(trimmed); ok {
				rows = append(rows, row)
			} else if len(rows) > 0 {
				// The dashes under the first row say it was the heading, and say nothing else.
				rows[len(rows)-1].Header = true
			}
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, ">"):
			flush()
			out = append(out, Text(node(), plain(strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))),
				TextBodyMuted, PaddingXY(0, 12)))
		case strings.HasPrefix(trimmed, "#"):
			flush()
			heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if !dropped && strings.HasPrefix(heading, item) {
				dropped = true
				continue
			}
			out = append(out, Text(node(), plain(heading), TextSubtitle))
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "), strings.HasPrefix(trimmed, "+ "):
			flush()
			listItem, listAt = []string{DocsBullet(plain(trimmed[2:]))}, indented(line)
		case numbered.MatchString(trimmed):
			// Kept exactly as written, number and all. A list whose items are numbered by the author
			// is numbered for a reason — the third step refers to the first — and renumbering it
			// here would be this server having an opinion about somebody else's document.
			//
			// It was a paragraph line until now, which is worse than it sounds: paragraph lines are
			// joined, so five steps became one sentence of a thousand characters, and a person
			// reading it saw the markup fail rather than a long paragraph. Measured on a live item
			// before the fix: 1091 characters in one node.
			flush()
			listItem, listAt = []string{plain(trimmed)}, indented(line)
		default:
			// A line with no marker of its own continues whatever is open. That is what a wrapped
			// line is, and a list item is the case where getting it wrong shows.
			if len(listItem) > 0 {
				listItem = append(listItem, plain(trimmed))
				continue
			}
			if len(rows) > 0 || len(code) > 0 {
				flush()
			}
			paragraph = append(paragraph, plain(trimmed))
		}
	}
	flush()
	return out
}

// tableRow reads one row of a markdown table, and reports the separator as not a row at all.
//
// Cells are plain strings by the wire type, so whatever is inside one — a link, a name in
// backticks — stays as the author typed it. That is the same limit a text node has and the reason
// the markup here stops at blocks.
func tableRow(line string) (TableRow, bool) {
	cells := strings.Split(strings.Trim(line, "|"), "|")
	separator := true
	row := TableRow{Cells: make([]string, 0, len(cells))}

	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		row.Cells = append(row.Cells, cell)
		if strings.Trim(cell, ":-") != "" || cell == "" {
			separator = false
		}
	}
	return row, !separator
}

// emphasis is what a source uses to make part of a line stand out.
var emphasis = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)

// plain takes the emphasis markers off a line.
//
// The line this draws is narrower than it looks. Emphasis is decoration: without it the sentence
// still says what it said, and with the asterisks left on it the reader is shown markup instead —
// which is what "it renders the markdown raw" means from the other side of the screen. A link is
// the opposite case and is left exactly as written: drawn as a link it would promise something this
// vocabulary cannot do (Q-73, kompot#56), and the address is the only part of it worth anything to
// somebody who cannot press it. Backticks stay for the same reason — they say "this is a name", and
// nothing else here can.
func plain(line string) string {
	return emphasis.ReplaceAllString(line, "$1$2")
}
