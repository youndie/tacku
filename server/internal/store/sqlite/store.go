// Package sqlite implements domain.Store on a single SQLite file.
//
// One file and no server process is a prototype decision (research D9): a team tracker at this
// stage has a handful of concurrent writers, and a separate database would add a step to every
// local run and a cgo toolchain to every build. The price is named there too — outgrowing it means
// rewriting this package, which is why nothing above internal/store knows it exists.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/youndie/tacku/server/internal/domain"
)

// Store is the SQLite implementation of domain.Store.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Open opens (and migrates) the database at path. Use ":memory:" for a throwaway one.
func Open(path string) (*Store, error) {
	// WAL so a reader never blocks the writer, and a busy timeout so a concurrent write waits
	// instead of failing outright — SQLITE_BUSY surfaces as a request that failed for no reason a
	// user can act on.
	//
	// _txlock=immediate is the one that is not obvious and was paid for. A transaction that begins
	// deferred takes a read lock and asks for the write lock later; if another writer got there in
	// between, SQLite answers "database is locked" straight away, because waiting could not resolve
	// it — the busy timeout does not apply to that upgrade. Every transaction here writes, and
	// several read first to refuse an unknown board or task by name, so they all take the write
	// lock up front instead.
	dsn := path + "?_txlock=immediate&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	// An in-memory database belongs to its connection, and database/sql keeps a pool: with more
	// than one connection the second query lands in a different, empty database and the failure
	// reads as data vanishing rather than as a configuration mistake.
	if path == ":memory:" || strings.Contains(path, "mode=memory") {
		db.SetMaxOpenConns(1)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Store) Close() error { return s.db.Close() }

const stamp = time.RFC3339Nano

func (s *Store) stampNow() string { return s.now().Format(stamp) }

func parseStamp(value string) time.Time {
	t, err := time.Parse(stamp, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// write runs fn inside a transaction. Every mutation goes through here, which is what makes "the
// change and its journal entry land together" a property of the package rather than a habit.
func (s *Store) write(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) Boards(ctx context.Context) ([]domain.Board, error) {
	rows, err := s.db.QueryContext(ctx, `select id, title from boards order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []domain.Board
	for rows.Next() {
		var b domain.Board
		if err := rows.Scan(&b.ID, &b.Title); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func (s *Store) CreateBoard(ctx context.Context, title string) (domain.Board, error) {
	if title == "" {
		return domain.Board{}, fmt.Errorf("%w: board needs a title", domain.ErrInvalidTask)
	}
	board := domain.Board{ID: domain.BoardID(title), Title: title}
	err := s.write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `insert into boards (id, title) values (?, ?)`, string(board.ID), title)
		return err
	})
	if err != nil {
		return domain.Board{}, err
	}
	return board, nil
}

func (s *Store) Task(ctx context.Context, id domain.TaskID) (domain.Task, error) {
	return scanTaskFor(s.db.QueryRowContext(ctx, taskColumns+` from tasks where id = ?`, string(id)), id)
}

func (s *Store) Tasks(ctx context.Context, board domain.BoardID) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, taskColumns+` from tasks where board = ? order by id`, string(board))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		task, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

const taskColumns = `select id, board, title, body, status, assignee, due, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanTask(row scanner) (domain.Task, error) {
	return scanTaskFor(row, "")
}

// scanTaskFor names the identifier in the refusal. An agent that mistyped one can act on "no task
// TAC-9999"; it cannot act on "not found".
func scanTaskFor(row scanner, id domain.TaskID) (domain.Task, error) {
	task, err := scanTaskRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		if id != "" {
			return domain.Task{}, fmt.Errorf("%w: no task %q", domain.ErrNotFound, string(id))
		}
		return domain.Task{}, fmt.Errorf("%w: task", domain.ErrNotFound)
	}
	return task, err
}

func scanTaskRow(row scanner) (domain.Task, error) {
	var t domain.Task
	var created, updated string
	err := row.Scan(&t.ID, &t.Board, &t.Title, &t.Body, &t.Status, &t.Assignee, &t.Due, &created, &updated)
	if err != nil {
		return domain.Task{}, err
	}
	t.CreatedAt = parseStamp(created)
	t.UpdatedAt = parseStamp(updated)
	return t, nil
}

// compile-time proof that this package satisfies the contract the rest of the server codes against.
var _ domain.Store = (*Store)(nil)
