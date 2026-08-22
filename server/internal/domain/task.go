package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Status is where a task sits on a board.
type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusInReview   Status = "in_review"
	StatusDone       Status = "done"
	StatusBlocked    Status = "blocked"
)

// Statuses is the closed set, in board order.
//
// There is no transition table on purpose. A tracker that forbids moving a task from Done back to
// In progress is a tracker people work around, and every workaround costs the history a real entry.
// What is closed is the set of names, because a status nobody declared cannot be rendered.
var Statuses = []Status{StatusTodo, StatusInProgress, StatusInReview, StatusDone, StatusBlocked}

// Valid reports whether the status is one this build knows.
func (s Status) Valid() bool { return slices.Contains(Statuses, s) }

var (
	ErrInvalidTask = errors.New("domain: invalid task")
	ErrNotFound    = errors.New("domain: not found")
	ErrConflict    = errors.New("domain: conflict")
)

// Task is the unit of work.
//
// Due is a date without a time zone, held as the ISO string the wire carries, because the toolkit
// has no date field and the prototype sends it through a text input with a pattern. Parsing it into
// a time.Time here would invent a precision the product does not have — see B-12.
type Task struct {
	ID        TaskID
	Board     BoardID
	Title     string
	Body      string
	Status    Status
	Assignee  MemberID
	Due       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DuePattern is the one format the product accepts, and the same one the form's regex rule carries.
const DuePattern = "2006-01-02"

// Validate checks what the store must never hold.
func (t Task) Validate() error {
	if !t.ID.Valid() {
		return fmt.Errorf("%w: %q is not a task identifier", ErrInvalidTask, string(t.ID))
	}
	if t.Board == "" {
		return fmt.Errorf("%w: no board", ErrInvalidTask)
	}
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("%w: empty title", ErrInvalidTask)
	}
	if !t.Status.Valid() {
		return fmt.Errorf("%w: unknown status %q", ErrInvalidTask, string(t.Status))
	}
	if t.Due != "" {
		if _, err := time.Parse(DuePattern, t.Due); err != nil {
			return fmt.Errorf("%w: due date %q is not %s", ErrInvalidTask, t.Due, DuePattern)
		}
	}
	return nil
}

// Board groups tasks.
type Board struct {
	ID    BoardID
	Title string
}

// Comment is a note a person or an agent left on a task.
type Comment struct {
	ID        int64
	Task      TaskID
	Text      string
	By        Provenance
	CreatedAt time.Time
}

// MoveOutcome is what happened to one task of a move that named several.
//
// There are two of them and neither is a failure, which follows from the operation being
// all-or-nothing: a task that cannot move takes the whole move down with it, so nothing survives to
// be reported as failed. A task already standing in the target status is not a failure either — it
// is the no-op every single-field edit already refuses to journal.
type MoveOutcome string

const (
	MoveMoved     MoveOutcome = "moved"
	MoveUnchanged MoveOutcome = "unchanged"
)

// MissingTasks is the refusal of a move that named tasks the store does not have.
//
// A type rather than a formatted ErrNotFound because the identifiers have to survive the trip out.
// What a person is told when a bulk action does not apply is the whole of B-32, and the answer there
// is that the refusal names what stopped it — which is impossible once the names have been baked
// into a sentence written for a log (§14).
//
// Every one of them and not the first: a refusal naming one of two sends the person back to correct
// half the selection, apply it, and meet the other half on the second attempt. The store therefore
// looks at everything that was named before it writes anything, which it has to do anyway — the
// operation is all-or-nothing.
type MissingTasks struct {
	Tasks []TaskID
}

func (e *MissingTasks) Error() string {
	names := make([]string, 0, len(e.Tasks))
	for _, id := range e.Tasks {
		names = append(names, string(id))
	}
	return fmt.Sprintf("%v: no tasks %s", ErrNotFound, strings.Join(names, ", "))
}

// Unwrap keeps the class of the refusal readable by everything that asks about it in the usual way,
// so a caller that does not care which tasks are gone goes on treating it as ErrNotFound.
func (e *MissingTasks) Unwrap() error { return ErrNotFound }

// MoveResult is what happened to one task, in the order the caller named them.
//
// A list rather than a count, and that is deliberate room rather than generosity: what a person is
// shown when a bulk action does not apply to everything is an open question (B-32), and a count
// cannot be turned back into "these two did not move" once the detail is gone.
type MoveResult struct {
	Task    TaskID
	From    Status
	To      Status
	Outcome MoveOutcome
}
