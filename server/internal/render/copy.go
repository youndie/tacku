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

// dayLayoutPattern is the same day in the spelling a client understands.
//
// Go says a date by example and everyone else by pattern letters, so one rendering needs two
// spellings. They sit together because the failure of letting them drift is a date shown two ways
// on two screens — the drift this file exists to prevent, in the one place the file cannot prevent
// it by holding a single string.
const dayLayoutPattern = "d MMM"

// AgentWord is the provenance signal written out, and it is the one channel that survives every way
// of not seeing a colour.
//
// Three marks say "a program did this": the stripe, the colour of the byline, the word. Measured
// rather than assumed (scripts/token_contrast.py): the stripe holds up — 4.9:1 against the human
// placeholder in the dark theme and 3.8:1 in the light one, against a threshold of 3:1, and a wide
// colour difference under both dichromat simulations. The colour of the byline does not hold up on
// its own — `meta_agent` against `meta` is 1.23:1 and 1.04:1, so on a greyscale screen the two
// bylines are the same grey; it survives protanopia and deuteranopia only because orange against
// grey lies on the blue-yellow axis, which those two leave alone.
//
// The word survives all of it and costs nothing. It is a constant rather than a literal in two
// files so that the check which guards it (TestNoAgentStripeIsTheOnlyCarrierOfItsMeaning) is
// looking at what the screens actually send.
const AgentWord = "Agent"

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
		return fmt.Sprintf("%s · on behalf of %s · %s", AgentWord, change.By.OnBehalfOf, at)
	}
	return fmt.Sprintf("%s · %s", change.By.Executor.Member, at)
}

// FeedSummary is the catch-up headline: a finished sentence, not a template.
//
// The server resolves the language and the plural because the client never assembles text, so
// "1 change" and "14 changes" are decided here.
func FeedSummary(total, boards int, away time.Duration) string {
	line := fmt.Sprintf("%d %s across %d %s",
		total, plural(total, "change", "changes"),
		boards, plural(boards, "board", "boards"))
	if since := lastVisit(away); since != "" {
		line += " · " + since
	}
	return line
}

// lastVisit says when the reader was last here, and says it as an elapsed duration rather than as
// the date and time the design drew.
//
// Not a compromise on the design but the only sentence that can be true. §14 hands the server the
// finished text, and nothing in the contract carries the reader's timezone — so "on 20 Aug at
// 18:40" would be the server's own wall clock wearing the reader's: plausible, unfalsifiable from
// the screen, and wrong by however many hours separate them (Q-31). How long ago it was is the same
// number everywhere.
//
// Rounded to the nearest hour or day rather than truncated: "9 hours ago" for eight hours and
// fifty-five minutes is the answer a person would give, and truncation would say eight.
func lastVisit(away time.Duration) string {
	switch {
	case away <= 0:
		return ""
	case away < time.Hour:
		return "you were last here under an hour ago"
	case away < 48*time.Hour:
		return "you were last here " + count(int((away+30*time.Minute)/time.Hour), "hour", "hours") + " ago"
	default:
		return "you were last here " + count(int((away+12*time.Hour)/(24*time.Hour)), "day", "days") + " ago"
	}
}

// count is a number with its unit agreed, which is the other half of what plural does.
func count(value int, one, many string) string {
	return fmt.Sprintf("%d %s", value, plural(value, one, many))
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

// CardMeta is the second line of a board card: which task, whose, by when — and, when an agent was
// the last to touch it, that as well.
//
// The line used to carry one of the two rather than both. A card an agent had touched read
// "Agent · on behalf of anna" and no longer said which task it was, who owned it or when it was due
// — the three facts a board is read for. Nothing failed and nothing looked broken: the line was in
// its place, saying something else, and the only way to notice was to hold two cards side by side.
//
// Four facts do not fit one line, so the decision this line needs is what to leave out. How much a
// line does fit is not something the contract answers — `text` carries a string and a token, with no
// number of lines, no overflow and no way back from the screen (Q-54) — so the line is shortened by
// what it says rather than by counting characters, and what is left out is the name of the person
// the agent acted for:
//
//   - The card already names a person — the assignee — so the byline's name is a second person in
//     the same line, and it is the same person twice whenever an agent works on its principal's own
//     task.
//   - The sentence it comes from is not lost: the feed and the task's own history carry Author,
//     where "on behalf of" is written out in full beside the time, and both are one tap from the
//     card. This line does not have that room; those do.
//
// Refused: a second line under the card, carrying the byline whole. A card's height is what decides
// how many of them a column shows, and provenance is not the exception here — an agent is a member
// of the team rather than a rare event, so the taller card would be the ordinary card.
//
// The mark goes in front. The stripe it belongs to is on the left edge of the same row, and the word
// qualifies everything after it rather than one of the facts.
//
// Whether the mark appears at all is decided here and not at the card, because the same choice is
// made on every list in the product: it was written out at the board and nowhere else once, and the
// rows of a filtered list — same product, same promise — said nothing about the agent at all.
func CardMeta(task domain.Task, by domain.Provenance) string {
	meta := string(task.ID) + " · " + AssigneeValue(task.Assignee)
	if task.Due != "" {
		meta += " · due " + day(task.Due)
	}
	if by.ByAgent() {
		return AgentWord + " · " + meta
	}
	return meta
}

// RowMeta is the same line on a list that crosses columns, so it also says where the task stands: a
// row that does not is a row the reader has to open to place.
func RowMeta(task domain.Task, by domain.Provenance) string {
	return CardMeta(task, by) + " · " + StatusName(string(task.Status))
}

// HiddenFromSelection says how much of the list a bulk move cannot see, with its verb agreed.
//
// Agreement is why this is here rather than beside the element: "1 more task is" and "4 more tasks
// are" are the same sentence choosing its words from a number, and a phrase chosen by data is what
// this file exists to hold.
func HiddenFromSelection(hidden int) string {
	return fmt.Sprintf("%d more %s not shown here. Move these first, or move them one at a time from the board.",
		hidden, plural(hidden, "task is", "tasks are"))
}

// MissingFromSelection is the refusal a bulk move answers when part of what was ticked is gone.
//
// The one message of this operation that a person actually reads. A bulk move either applies whole
// or refuses whole, which leaves exactly one outcome worth a sentence — and a refusal is also the
// only answer the protocol carries text in at all: the actions of the profile carry none, and the
// body of §16.8 hangs off a refusal (Q-35).
//
// So the sentence has work to do. It names every task that stopped the move rather than counting
// them, because a count cannot be acted on; it says that nothing moved, because the person is about
// to look at a board and has to know whether to look for changes; and it says what to do next,
// because the screen they came from is out of date by exactly this much.
//
// Here rather than beside the handler for the ordinary reason: the number of names chooses the verb,
// and a phrase chosen by data is a grammar the next handler would otherwise instantiate in its own
// words.
func MissingFromSelection(tasks []domain.TaskID) string {
	names := make([]string, 0, len(tasks))
	for _, id := range tasks {
		names = append(names, string(id))
	}
	return fmt.Sprintf("%s %s no longer there, so nothing moved. Open the screen again and choose from what is left.",
		series(names), plural(len(names), "is", "are"))
}

// series is a list of names the way a person writes one: one name alone, two joined by a word,
// more with commas before the last.
//
// A helper of its own rather than a strings.Join, because the join is not the same at every length,
// and a sentence reading “TAC-2, TAC-7 are no longer there” is the kind of thing a reader notices
// and a test never does.
func series(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// WizardFinishLabel is the word on the button that ends the only flow this product has.
//
// It lives here because it is a string on a screen, and this file is where those live. It says what
// the step does rather than that the step is the last one: "Finish" is true of every flow ever
// written and tells a person nothing about what is about to happen.
func WizardFinishLabel() string { return "Create the task" }

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
