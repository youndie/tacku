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
	MoveTask(ctx context.Context, id TaskID, to Status, by Provenance) (Task, error)
	AssignTask(ctx context.Context, id TaskID, to MemberID, by Provenance) (Task, error)
	SetDue(ctx context.Context, id TaskID, due string, by Provenance) (Task, error)

	Comment(ctx context.Context, id TaskID, text string, by Provenance) (Comment, error)
	Comments(ctx context.Context, id TaskID) ([]Comment, error)

	// Changes returns what happened after the cursor, oldest first, and the cursor to come back
	// with. Limit bounds the page; the returned cursor advances only over what was actually
	// returned, so a reader that stops early does not skip the remainder.
	Changes(ctx context.Context, after Cursor, limit int) ([]Change, Cursor, error)

	// Latest is the cursor standing after everything written so far.
	Latest(ctx context.Context) (Cursor, error)

	Close() error
}
