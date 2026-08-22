package render_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/render"
)

// A kind that no build has ever recorded, used to read the fallback out of the renderer rather than
// copying it into the test. Comparing against a repeated literal would keep passing after somebody
// changed the fallback, which is the one string here that must not be locked down.
const unknownKind domain.ChangeKind = "nothing_this_build_knows"

// Every kind is phrased, in both places, and adding one cannot be a one-line edit.
//
// This is the acceptance criterion of B-25 as a check: the journal has two phrasings per kind — one
// for the feed, which names the task, and one for the task's own history, which does not — and
// neither the compiler nor a switch with a default says a word when a new kind arrives with no
// phrasing of its own. It renders "Changed “TAC-1”", which is a sentence, so nothing looks broken.
//
// The list is walked twice on purpose. The const block is read out of the source because a kind
// declared there and left out of domain.ChangeKinds would otherwise slip past both the list and
// this test — the gate would be guarding a door with a second door beside it.
func TestEveryDeclaredKindIsPhrasedInBothPlaces(t *testing.T) {
	declared := kindsInSource(t)
	listed := map[domain.ChangeKind]bool{}
	for _, kind := range domain.ChangeKinds {
		listed[kind] = true
	}

	t.Logf("%d kinds declared in the source, %d listed in domain.ChangeKinds, %d phrasings expected",
		len(declared), len(listed), 2*len(listed))
	if len(declared) == 0 {
		t.Fatal("no ChangeKind was found in the source, so this check read the wrong file")
	}

	for _, kind := range declared {
		if !listed[kind] {
			t.Errorf("the kind %q is declared and missing from domain.ChangeKinds, so nothing walks it: "+
				"it renders the fallback in the feed and in the history, and nothing says so", kind)
		}
	}
	for kind := range listed {
		if !contains(declared, kind) {
			t.Errorf("domain.ChangeKinds lists %q, which is not declared: the list and the vocabulary "+
				"have parted", kind)
		}
	}

	fallbackSentence := render.Sentence(change(unknownKind))
	fallbackHistory := render.HistoryLine(change(unknownKind))

	for _, kind := range domain.ChangeKinds {
		if got := render.Sentence(change(kind)); got == fallbackSentence {
			t.Errorf("the feed phrases %q as the fallback %q: the event has no sentence of its own",
				kind, got)
		}
		if got := render.HistoryLine(change(kind)); got == fallbackHistory {
			t.Errorf("a task's history phrases %q as the fallback %q: the event has no line of its own",
				kind, got)
		}
	}
}

// The fourteen phrasings, written out.
//
// Seven kinds in two places, which is the count the design arrived at and the reason this file
// exists: a feed line names the task because the reader is scanning a list, and the same event on
// the task's own screen does not because the reader is already looking at it.
//
// Locked down as text rather than described, because the three rules underneath them are invisible
// in a description: "Moved" is not reused for a due date, no status name is baked into a phrase, and
// a comment is cut to one line. A test that only checked "some string comes back" would pass through
// every one of those being broken.
func TestTheFourteenPhrasings(t *testing.T) {
	moved := change(domain.ChangeStatusMoved)
	moved.From, moved.To = string(domain.StatusInProgress), string(domain.StatusInReview)

	created := change(domain.ChangeTaskCreated)
	created.To = "Ship the thing"

	assigned := change(domain.ChangeAssigned)
	assigned.To = "anna"

	due := change(domain.ChangeDueChanged)
	due.From, due.To = "2026-08-26", "2026-08-28"

	renamed := change(domain.ChangeTitleEdited)
	renamed.From, renamed.To = "Old name", "New name"

	commented := change(domain.ChangeCommentPosted)
	commented.To = "Looks right to me"

	cases := []struct {
		name    string
		change  domain.Change
		feed    string
		history string
	}{
		{
			name:    "created",
			change:  created,
			feed:    "Created “Ship the thing”",
			history: "Created",
		},
		{
			// The status names come from the vocabulary, never from the phrase. A column renamed
			// Shipped must not turn old entries into lies — records are immutable.
			name:    "moved",
			change:  moved,
			feed:    "Moved “TAC-1” from In progress to In review",
			history: "Moved from In progress to In review",
		},
		{
			name:    "assigned",
			change:  assigned,
			feed:    "Assigned “TAC-1” to anna",
			history: "Assigned to anna",
		},
		{
			// "Changed", not "Moved": the verb belongs to status, and two meanings on one verb is
			// how a journal stops being read.
			name:    "due date changed",
			change:  due,
			feed:    "Changed the due date of “TAC-1” from 26 Aug to 28 Aug",
			history: "Due date changed from 26 Aug to 28 Aug",
		},
		{
			name:    "renamed",
			change:  renamed,
			feed:    "Renamed “Old name” to “New name”",
			history: "Renamed to “New name”",
		},
		{
			// No values: a description is a paragraph, and "from … to …" in a feed is unreadable.
			name:    "description edited",
			change:  change(domain.ChangeBodyEdited),
			feed:    "Edited the description of “TAC-1”",
			history: "Description edited",
		},
		{
			name:    "commented",
			change:  commented,
			feed:    "Commented on “TAC-1”: “Looks right to me”",
			history: "Commented: “Looks right to me”",
		},
	}

	if len(cases) != len(domain.ChangeKinds) {
		t.Fatalf("%d phrasings are written out and the vocabulary has %d kinds",
			len(cases), len(domain.ChangeKinds))
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := render.Sentence(one.change); got != one.feed {
				t.Errorf("the feed line is %q, want %q", got, one.feed)
			}
			if got := render.HistoryLine(one.change); got != one.history {
				t.Errorf("the history line is %q, want %q", got, one.history)
			}
		})
	}
}

// A third verb for setting a date from nothing, and a fourth for taking one away.
//
// Kept out of the table above because they are branches of one kind rather than kinds, and it is the
// verbs that matter: "Set" is not "Changed" and neither is "Moved", which is the discipline the
// design asked for in the one place it is easiest to lose.
func TestADueDateGetsItsOwnVerbs(t *testing.T) {
	set := change(domain.ChangeDueChanged)
	set.To = "2026-08-28"

	removed := change(domain.ChangeDueChanged)
	removed.From = "2026-08-28"

	if got, want := render.Sentence(set), "Set the due date of “TAC-1” to 28 Aug"; got != want {
		t.Errorf("setting a date from nothing reads %q, want %q", got, want)
	}
	if got, want := render.Sentence(removed), "Removed the due date from “TAC-1”"; got != want {
		t.Errorf("removing a date reads %q, want %q", got, want)
	}
	if strings.HasPrefix(render.Sentence(set), "Moved") {
		t.Error("a due date is phrased with the verb that belongs to status")
	}
}

// A comment is cut to one line, and the cut is the server's.
//
// 120 characters, from the design: a feed row carries one line, and a paragraph pasted into a
// comment would push everything under it off the screen. The client cannot do this — it renders
// finished text — so a comment that arrives whole is a comment shown whole.
func TestACommentIsCutToOneLineByTheServer(t *testing.T) {
	long := change(domain.ChangeCommentPosted)
	long.To = strings.Repeat("word ", 60) + "\n\nand a second paragraph"

	sentence := render.Sentence(long)
	if !strings.Contains(sentence, "…") {
		t.Errorf("a 300-character comment arrives in the feed uncut: %q", sentence)
	}
	if strings.Contains(sentence, "second paragraph") {
		t.Error("a paragraph past the cut reaches the feed")
	}
	if count := len([]rune(sentence)); count > 160 {
		t.Errorf("the feed line is %d runes long; a row carries one line", count)
	}
}

// The stand-ins shown where a value is absent, in one voice.
//
// Two of the three used to be written twice: a board card said "unassigned" and the task beside it
// said "Unassigned". Nobody would have noticed without opening the two files on the same day, which
// is the whole argument for the file this checks.
func TestAnAbsentValueIsSpelledOneWay(t *testing.T) {
	task := domain.Task{ID: "TAC-1", Status: domain.StatusTodo}
	card := render.CardMeta(task, domain.Human("anna"))
	sidebar := render.AssigneeValue(task.Assignee)

	if !strings.Contains(card, sidebar) {
		t.Errorf("a card says %q of a task nobody owns and the task screen says %q", card, sidebar)
	}
	if got, want := render.DueValue(""), "No due date"; got != want {
		t.Errorf("an absent due date reads %q, want %q", got, want)
	}
	if got, want := render.DescriptionValue(""), "No description yet."; got != want {
		t.Errorf("an absent description reads %q, want %q", got, want)
	}
}

// Counts carry their plural, because the client cannot.
func TestACountCarriesItsPlural(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{render.FeedSummary(1, 1, 0), "1 change across 1 board"},
		{render.FeedSummary(25, 2, 0), "25 changes across 2 boards"},
		// Away is part of the same sentence, so it agrees in the same place.
		{render.FeedSummary(25, 2, 90*time.Minute), "25 changes across 2 boards · you were last here 2 hours ago"},
		{render.FeedSummary(1, 1, 61*time.Minute), "1 change across 1 board · you were last here 1 hour ago"},
		{render.FeedSummary(1, 1, 30*time.Minute), "1 change across 1 board · you were last here under an hour ago"},
		{render.BoardSummary(1, 0), "1 open task · 0 changed since your last visit"},
		{render.BoardSummary(4, 3), "4 open tasks · 3 changed since your last visit"},
	}
	for _, one := range cases {
		if one.got != one.want {
			t.Errorf("the headline reads %q, want %q", one.got, one.want)
		}
	}
}

// A destination is named once, by the graph.
//
// The navigation panel used to spell its own captions, and the two spellings had already parted:
// the graph carried "Board" while the button beside it said "Boards". Neither was wrong on its own,
// which is why it survived.
func TestADestinationIsNamedByTheGraph(t *testing.T) {
	for _, route := range render.Graph {
		if got := render.RouteTitle(route.Deeplink); got != route.Title {
			t.Errorf("the graph calls %s %q and the renderer calls it %q", route.Deeplink, route.Title, got)
		}
	}
	if len(render.Graph) == 0 {
		t.Fatal("the graph is empty, so this check compared nothing")
	}
}

func change(kind domain.ChangeKind) domain.Change {
	return domain.Change{
		Seq:       1,
		Task:      "TAC-1",
		Board:     "BRD-1",
		Kind:      kind,
		By:        domain.Human("anna"),
		CreatedAt: time.Date(2026, 8, 21, 9, 30, 0, 0, time.UTC),
	}
}

func contains(kinds []domain.ChangeKind, kind domain.ChangeKind) bool {
	for _, one := range kinds {
		if one == kind {
			return true
		}
	}
	return false
}

// kindsInSource reads the const block itself, so that the list this package walks can be compared
// against the vocabulary rather than against itself.
func kindsInSource(t *testing.T) []domain.ChangeKind {
	t.Helper()

	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, filepath.Join("..", "domain", "change.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var kinds []domain.ChangeKind
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		named, ok := spec.Type.(*ast.Ident)
		if !ok || named.Name != "ChangeKind" {
			return true
		}
		for _, value := range spec.Values {
			lit, ok := value.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			unquoted, err := strconv.Unquote(lit.Value)
			if err != nil {
				continue
			}
			kinds = append(kinds, domain.ChangeKind(unquoted))
		}
		return true
	})
	return kinds
}
