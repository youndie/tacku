package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

// The words the journal is read in.
//
// One file, because the server sends finished text and the client never assembles any: every
// phrasing is code, and code written wherever a handler happened to need it becomes five styles of
// one sentence within a month.
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
	// learns that something happened and who did it.
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

// day turns an ISO date into the form a person reads. An unparseable one is passed through rather
// than hidden: a wrong date visible is worth more than a right-looking blank.
func day(iso string) string {
	parsed, err := time.Parse(domain.DuePattern, iso)
	if err != nil {
		return iso
	}
	return parsed.Format("2 Jan")
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
