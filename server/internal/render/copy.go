package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

// The words the server puts on data.
//
// The server sends finished text and the client assembles none (§14), so every phrasing is code —
// and code written wherever a handler happened to need it becomes five styles of one sentence
// within a month. The line this file draws is not "all text" but a narrower one that can be held:
//
//   - Text *about data* lives here: the journal's phrasings, the name of a status, a date, a count
//     with its plural, a line of authorship, a card's meta line, the stand-in shown where a value is
//     absent, and any label whose words depend on a value ("Move to In review"). Take the data away
//     and the text changes or disappears.
//
//   - Text *about a control* stays at the element that shows it: a screen title, a button caption, a
//     field label, a placeholder, the message of a rule, the empty state of a whole screen. It
//     exists in exactly one place, it is read together with the layout around it, and a rule's
//     message moved away from the rule drifts from the constraint it describes.
//
//   - Text addressed to a model — MCP tool descriptions, the text of an execution error — is not
//     interface copy at all and is governed by the tool contract instead. One owner over two
//     vocabularies with opposite requirements would be a rule nobody could follow.
//
// The reason for the split is drift, not tidiness: an assembled phrase is a grammar the next handler
// will instantiate again, while a written caption has one instance and is checked by looking at the
// screen. A grammar needs an owner; a caption needs a place.
//
// The line is enforced by copy_place_test.go rather than by convention, because the failure this
// file exists to prevent is somebody adding a phrase without reading this comment.
//
// Three rules come from the design and are worth keeping past this product:
//
//   - A verb is not reused. "Moved" belongs to status; a due date is "Changed", and setting one
//     from nothing is "Set". Two meanings on one verb is how a journal stops being read.
//   - A status name is never baked into a phrase. "Closed" was refused because a column may be
//     renamed Shipped, and the entry would become a lie nobody rewrites — records are immutable.
//   - There are fourteen phrasings, not seven. A feed line names the task; the same event on the
//     task's own history does not, because the reader is already looking at it.
const commentLimit = 120

// dayLayout is how a date is shown, in one place: two renderings of the same day on two screens is
// the smallest version of the drift this file exists to prevent.
const dayLayout = "2 Jan"

// Sentence is the feed line: the event, naming the task.
func Sentence(change domain.Change) string {
	switch change.Kind {
	case domain.ChangeTaskCreated:
		// On creation the journal carries the title rather than the identifier: the entry is what
		// gives the task its name in the first place.
		return fmt.Sprintf("Created %s", quote(change.To))

	case domain.ChangeStatusMoved:
		return fmt.Sprintf("Moved %s from %s to %s",
			quote(change.Task.String()), StatusName(change.From), StatusName(change.To))

	case domain.ChangeAssigned:
		if change.To == "" {
			return fmt.Sprintf("Removed %s from %s", change.From, quote(change.Task.String()))
		}
		return fmt.Sprintf("Assigned %s to %s", quote(change.Task.String()), change.To)

	case domain.ChangeDueChanged:
		switch {
		case change.To == "":
			return fmt.Sprintf("Removed the due date from %s", quote(change.Task.String()))
		case change.From == "":
			return fmt.Sprintf("Set the due date of %s to %s", quote(change.Task.String()), day(change.To))
		default:
			return fmt.Sprintf("Changed the due date of %s from %s to %s",
				quote(change.Task.String()), day(change.From), day(change.To))
		}

	case domain.ChangeTitleEdited:
		return fmt.Sprintf("Renamed %s to %s", quote(change.From), quote(change.To))

	case domain.ChangeBodyEdited:
		// No values, deliberately. A description is a paragraph, and "changed it from … to …" in a
		// feed is unreadable; the detail is one tap away on the task.
		return fmt.Sprintf("Edited the description of %s", quote(change.Task.String()))

	case domain.ChangeCommentPosted:
		return fmt.Sprintf("Commented on %s: %s", quote(change.Task.String()), quote(clip(change.To)))
	}

	// An event this build does not phrase. Better a flat line than a blank one: the reader still
	// learns that something happened and who did it. A kind of *this* build reaching it is a defect,
	// which is why the test compares every declared kind against exactly this string.
	return fmt.Sprintf("Changed %s", quote(change.Task.String()))
}

// HistoryLine is the same event on the task's own screen, where the task needs no naming.
func HistoryLine(change domain.Change) string {
	switch change.Kind {
	case domain.ChangeTaskCreated:
		return "Created"

	case domain.ChangeStatusMoved:
		return fmt.Sprintf("Moved from %s to %s", StatusName(change.From), StatusName(change.To))

	case domain.ChangeAssigned:
		if change.To == "" {
			return fmt.Sprintf("Removed %s as the assignee", change.From)
		}
		return fmt.Sprintf("Assigned to %s", change.To)

	case domain.ChangeDueChanged:
		switch {
		case change.To == "":
			return "Due date removed"
		case change.From == "":
			return fmt.Sprintf("Due date set to %s", day(change.To))
		default:
			return fmt.Sprintf("Due date changed from %s to %s", day(change.From), day(change.To))
		}

	case domain.ChangeTitleEdited:
		return fmt.Sprintf("Renamed to %s", quote(change.To))

	case domain.ChangeBodyEdited:
		return "Description edited"

	case domain.ChangeCommentPosted:
		return fmt.Sprintf("Commented: %s", quote(clip(change.To)))
	}

	return "Changed"
}

// Author is the second half of provenance: who acted, and for whom.
func Author(change domain.Change) string {
	at := change.CreatedAt.Format("15:04")
	if change.By.ByAgent() {
		return fmt.Sprintf("Agent · on behalf of %s · %s", change.By.OnBehalfOf, at)
	}
	return fmt.Sprintf("%s · %s", change.By.Executor.Member, at)
}

// agentMeta is what a list item says instead of its usual meta line when an agent touched it.
//
// One phrasing shared with Author on purpose: a reader who learns what "on behalf of" means in the
// feed should not have to learn it again on a board.
func agentMeta(by domain.Provenance) string {
	return fmt.Sprintf("Agent · on behalf of %s", by.OnBehalfOf)
}

// FeedSummary is the catch-up headline: a finished sentence, not a template.
//
// The server resolves the language and the plural because the client never assembles text, so
// "1 change" and "14 changes" are decided here.
func FeedSummary(total, boards int) string {
	return fmt.Sprintf("%d %s across %d %s",
		total, plural(total, "change", "changes"),
		boards, plural(boards, "board", "boards"))
}

// BoardSummary is the line under a board's name.
func BoardSummary(open, changed int) string {
	return fmt.Sprintf("%d open %s · %d changed since your last visit",
		open, plural(open, "task", "tasks"), changed)
}

// ColumnHeading is a status as a column head.
func ColumnHeading(status domain.Status) string { return upper(StatusName(string(status))) }

// EmptyColumnLine is one line and no call to action.
//
// Four columns empty on a Monday morning would be four invitations on one screen, and the heading
// already says the column is empty — the line says why that is fine. It is here rather than beside
// the column it fills because the words are chosen by the status, and a phrase chosen by data is
// a phrase that will be chosen again by whoever adds the next status.
func EmptyColumnLine(status domain.Status) string {
	if status == domain.StatusDone {
		return "Nothing finished yet this sprint."
	}
	return "Nothing here yet."
}

// MoveLabel is the caption of the single-step button on a board card.
func MoveLabel(target domain.Status) string {
	return fmt.Sprintf("Move to %s", StatusName(string(target)))
}

// BackLabel is the caption of the button that leaves a task for its board.
func BackLabel(board domain.BoardID) string { return fmt.Sprintf("← %s", board) }

// CardMeta is the second line of a board card: which task, whose, and by when — unless an agent
// touched it last, in which case the line says so instead.
//
// Which of the two a card shows is decided here and not at the card, because the same choice is made
// on every list in the product: it was written out at the board and nowhere else once, and the rows
// of a filtered list — same product, same promise — said nothing about the agent at all.
func CardMeta(task domain.Task, by domain.Provenance) string {
	if by.ByAgent() {
		return agentMeta(by)
	}
	meta := string(task.ID) + " · " + AssigneeValue(task.Assignee)
	if task.Due != "" {
		meta += " · due " + day(task.Due)
	}
	return meta
}

// RowMeta is the same line on a list that crosses columns, so it also says where the task stands: a
// row that does not is a row the reader has to open to place.
func RowMeta(task domain.Task, by domain.Provenance) string {
	return CardMeta(task, by) + " · " + StatusName(string(task.Status))
}

// TaskMeta is the line under a task's title.
func TaskMeta(task domain.Task) string {
	return fmt.Sprintf("%s · created %s", task.ID, task.CreatedAt.Format(dayLayout))
}

// AssigneeValue, DueValue and DescriptionValue are the three stand-ins shown where a value is
// absent. They are together because they are one decision — that an empty area says nothing about
// whether a value is missing or merely short — and because two of the three used to be written twice
// in different words: a card said "unassigned" while the task beside it said "Unassigned".
func AssigneeValue(member domain.MemberID) string {
	if member == "" {
		return "Unassigned"
	}
	return string(member)
}

// DueValue renders a due date, or its absence.
func DueValue(iso string) string {
	if iso == "" {
		return "No due date"
	}
	return day(iso)
}

// DescriptionValue renders a task's body, or its absence.
func DescriptionValue(body string) string {
	if body == "" {
		return "No description yet."
	}
	return body
}

// StatusName is the display name of a status, resolved here because the wire form is a key and the
// screen shows words.
func StatusName(status string) string {
	switch domain.Status(status) {
	case domain.StatusTodo:
		return "To do"
	case domain.StatusInProgress:
		return "In progress"
	case domain.StatusInReview:
		return "In review"
	case domain.StatusDone:
		return "Done"
	case domain.StatusBlocked:
		return "Blocked"
	}
	return status
}

// RouteTitle is what the navigation graph calls a destination.
//
// The panel used to spell its own captions, and the two spellings had already parted: the graph
// carried "Board" and the button beside it said "Boards". Nobody would have noticed until somebody
// read the two files on the same day.
func RouteTitle(deeplink string) string {
	for _, route := range Graph {
		if route.Deeplink == deeplink {
			return route.Title
		}
	}
	// A destination outside the graph has no title to borrow, and inventing one here would hide the
	// real fault — a button pointing at a deeplink the client cannot resolve.
	return deeplink
}

// plural picks the form. One place, so that the next count does not invent a second way of asking.
func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

// day turns an ISO date into the form a person reads. An unparseable one is passed through rather
// than hidden: a wrong date visible is worth more than a right-looking blank.
func day(iso string) string {
	parsed, err := time.Parse(domain.DuePattern, iso)
	if err != nil {
		return iso
	}
	return parsed.Format(dayLayout)
}

// clip keeps a comment to one line of a feed. Paragraphs belong on the task.
func clip(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) <= commentLimit {
		return body
	}
	return strings.TrimSpace(body[:commentLimit]) + "…"
}

func quote(value string) string { return "“" + value + "”" }

// upper is ASCII-only and deliberately so: it exists for column headings, whose words this file
// writes, and a general case would invite it to be used on data it must not touch.
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
