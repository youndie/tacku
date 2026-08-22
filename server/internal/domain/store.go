package domain

import "context"

// Store is everything the rest of the server may ask of storage.
//
// It is kept narrow on purpose, and the reason is not portability in the abstract. Two concrete
// futures depend on it: SQLite is a prototype decision that a real deployment will outgrow, and the
// backlog holds an open question about reading a docs-as-code backlog out of git as a second
// adapter behind this same interface. Neither survives if SQL leaks upwards.
//
// Every mutating method takes Provenance and writes the change and its journal entry in one
// transaction. That is the invariant the whole product rests on: a task cannot move without the
// journal knowing who moved it, because a state that changed without an entry is invisible to the
// agent's cursor for ever.
type Store interface {
	Boards(ctx context.Context) ([]Board, error)
	CreateBoard(ctx context.Context, title string) (Board, error)

	Task(ctx context.Context, id TaskID) (Task, error)
	Tasks(ctx context.Context, board BoardID) ([]Task, error)

	CreateTask(ctx context.Context, draft Task, by Provenance) (Task, error)
	// MoveTask takes the surface it was asked from, and refuses a call that does not name one.
	// Which of the two places a status changes in — the board or the task — is a product question
	// with a decision hanging on it, and an unnamed caller would answer it with a blank row.
	MoveTask(ctx context.Context, id TaskID, to Status, by Provenance, from Surface) (Task, error)
	AssignTask(ctx context.Context, id TaskID, to MemberID, by Provenance) (Task, error)
	SetDue(ctx context.Context, id TaskID, due string, by Provenance) (Task, error)

	Comment(ctx context.Context, id TaskID, text string, by Provenance) (Comment, error)
	Comments(ctx context.Context, id TaskID) ([]Comment, error)

	// Changes returns what happened after the cursor, oldest first, and the cursor to come back
	// with. Limit bounds the page; the returned cursor advances only over what was actually
	// returned, so a reader that stops early does not skip the remainder.
	Changes(ctx context.Context, after Cursor, limit int) ([]Change, Cursor, error)

	// CountSince is how much stands after a cursor, and across how many boards.
	//
	// Asked separately from Changes because the headline of the catch-up screen is about
	// everything waiting, and Changes hands back one page. Counting the page and calling it the
	// total is a number that looks measured, agrees with the list beside it, and is wrong by
	// however much did not fit.
	CountSince(ctx context.Context, after Cursor) (changes, boards int, err error)

	// LastActors is who last touched each task, by task.
	//
	// A query and not a walk over the journal: the board used to read the first 500 entries and
	// keep the last one it saw per task, which is right until the journal passes 500 and then
	// silently wrong — the newest changes stop being read at all, and the provenance stripe fades
	// off the busiest boards first. Nothing fails; the signal just stops being there.
	LastActors(ctx context.Context) (map[TaskID]Provenance, error)

	// TaskChanges is the history of one task, oldest first.
	//
	// A separate read rather than a filter over Changes: the journal is walked by cursor for the
	// two readers who follow it forward, and a task's own history is a different question asked of
	// the same rows.
	TaskChanges(ctx context.Context, id TaskID) ([]Change, error)

	// Latest is the cursor standing after everything written so far.
	Latest(ctx context.Context) (Cursor, error)

	Close() error
}

// Outcome is what a previous attempt produced, kept so that a repeat does not produce a second one.
type Outcome struct {
	RequestHash string
	Body        []byte
}

// Attempts records and replays the outcome of an attempt, keyed by the caller's idempotency key.
//
// Split out of Store so the interface reads as two responsibilities rather than one long list, and
// because a second adapter may well satisfy one of them and not the other.
type Attempts interface {
	Outcome(ctx context.Context, key string) (Outcome, bool, error)
	Remember(ctx context.Context, key string, outcome Outcome) error
}

// Seen is where a person had read up to.
//
// A cursor and not a timestamp, so that "what changed since" is one question with one answer for
// both readers: an agent asks it of its own cursor, a person of theirs. The extra fields of Visit
// are how the person's cursor moves on its own; the agent moves its own by naming it.
type Seen interface {
	// Visit is everything remembered about this person's reading. A person who has never been
	// here has a visit at the start of the journal rather than an error.
	Visit(ctx context.Context, member MemberID) (Visit, error)

	// RecordVisit stores it back, and it is the only way the row is written — the explicit "mark
	// all as seen" goes through Dismiss and lands here too.
	//
	// Read-then-write rather than one statement, because the rule that decides the new value is
	// Arrive, and it belongs to the domain rather than to SQL: storage that computed the boundary
	// would put the answer to B-27 in a place where no test can see it in isolation.
	//
	// One write path rather than two, and that is not tidiness. A second statement of its own
	// stamped the time from the storage clock while the arrival compared against the caller's, and
	// two clocks that are compared to each other are one clock with a bug in it: an explicit mark
	// timed from somewhere else could put the next arrival either side of the gap for reasons
	// nothing in the request explains.
	RecordVisit(ctx context.Context, member MemberID, visit Visit) error
}
