package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ChangeKind names what happened, in the vocabulary the feed renders.
type ChangeKind string

const (
	ChangeTaskCreated   ChangeKind = "task_created"
	ChangeStatusMoved   ChangeKind = "status_moved"
	ChangeAssigned      ChangeKind = "assigned"
	ChangeDueChanged    ChangeKind = "due_changed"
	ChangeTitleEdited   ChangeKind = "title_edited"
	ChangeBodyEdited    ChangeKind = "body_edited"
	ChangeCommentPosted ChangeKind = "comment_posted"
)

// ChangeKinds is the closed set, in the order the vocabulary was written.
//
// It exists so that adding a kind cannot be a one-line edit of the block above: the renderer has two
// phrasings per kind — one for the feed, one for the task's own history — and neither the compiler
// nor a switch with a default can say that a new kind arrived without them. The check that walks
// this list also walks the const block, so a kind declared and left out of the list is caught too.
var ChangeKinds = []ChangeKind{
	ChangeTaskCreated,
	ChangeStatusMoved,
	ChangeAssigned,
	ChangeDueChanged,
	ChangeTitleEdited,
	ChangeBodyEdited,
	ChangeCommentPosted,
}


// Surface names where a change was made from.
//
// It exists for one question and says so: the board's card button and the task screen's selector
// both move a task, and nothing in the entry told them apart, so "do people move tasks from the
// board or from inside the task" could not be answered even in principle. The protocol does not
// carry the originating screen in any request (Q-24), so the surface is what the server knows from
// the address the call arrived at, and it has to be written down at that moment or it is gone.
//
// Recorded only where two surfaces compete for the same change — status moves today. Empty on the
// other kinds means the question was not asked of them, not that the answer was lost.
type Surface string

const (
	// SurfaceNone is the surface of a change nobody asked the question about.
	SurfaceNone Surface = ""
	// SurfaceBoard is the button on a board card.
	SurfaceBoard Surface = "board"
	// SurfaceTask is the selector on the task screen.
	SurfaceTask Surface = "task"
	// SurfaceAgent is a tool call. An agent has no screen at all, which is why it is a surface of
	// its own rather than a blank: a share computed over "board or task" must be able to leave
	// agent moves out, and it cannot do that if they are indistinguishable from unrecorded ones.
	SurfaceAgent Surface = "agent"
)

// Named reports whether the surface is one this server knows.
//
// Checked rather than assumed, because the cost of a wrong answer here is silent: a third way to
// move a task — bulk change is the next one — would otherwise write blanks, and the share would
// stay a number, just no longer a true one.
func (s Surface) Named() bool {
	switch s {
	case SurfaceBoard, SurfaceTask, SurfaceAgent:
		return true
	default:
		return false
	}
}

// ErrUnnamedSurface is returned when a change that has to name its surface did not.
var ErrUnnamedSurface = errors.New("domain: the change does not name the surface it came from")

// Change is one entry of the journal.
//
// The journal is part of the core rather than an audit trail bolted on top, because three different
// readers need the same structure: an agent asking what moved since its cursor, a person catching up
// after a night away, and the history on a task. One shape for three consumers is reason enough to
// build it first instead of fitting it around finished tables later.
//
// From and To are rendered strings rather than typed fields. What the feed shows is "from In
// progress to In review" for a status and "from 26 Aug to 28 Aug" for a date, and a column per kind
// would grow with every kind that is ever added.
type Change struct {
	Seq       int64
	Task      TaskID
	Board     BoardID
	Kind      ChangeKind
	From      string
	To        string
	Surface   Surface
	By        Provenance
	CreatedAt time.Time
}

// Cursor is a client's position in the journal.
//
// Opaque on purpose. A handle that shows its structure invites parsing and arithmetic, and the
// moment a client computes cursor+1 the server can no longer change how positions are numbered.
type Cursor string

// ErrBadCursor is returned when a cursor did not come from this server.
var ErrBadCursor = errors.New("domain: cursor is not one of ours")

const cursorPrefix = "c"

// CursorAt builds the cursor that stands just after seq.
func CursorAt(seq int64) Cursor { return Cursor(cursorPrefix + strconv.FormatInt(seq, 10)) }

// Start is the cursor of a reader that has never looked.
const Start Cursor = "c0"

// Seq decodes a cursor.
//
// The encoding is deliberately trivial and deliberately private: it is checked here so that a
// fabricated cursor fails loudly instead of being read as zero and replaying the entire journal
// into an agent's context.
func (c Cursor) Seq() (int64, error) {
	if !strings.HasPrefix(string(c), cursorPrefix) {
		return 0, fmt.Errorf("%w: %q", ErrBadCursor, string(c))
	}
	seq, err := strconv.ParseInt(strings.TrimPrefix(string(c), cursorPrefix), 10, 64)
	if err != nil || seq < 0 {
		return 0, fmt.Errorf("%w: %q", ErrBadCursor, string(c))
	}
	return seq, nil
}
