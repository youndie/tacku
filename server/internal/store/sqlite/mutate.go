package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/youndie/tacku/server/internal/domain"
)

// record appends the journal entry. Called only from inside a transaction that also performs the
// change, which is the invariant domain.Store states.
func (s *Store) record(ctx context.Context, tx *sql.Tx, c domain.Change) error {
	_, err := tx.ExecContext(ctx,
		`insert into changes (task, board, kind, from_value, to_value, surface,
		                      actor_kind, actor_member, actor_version, on_behalf_of, created_at)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(c.Task), string(c.Board), string(c.Kind), c.From, c.To, string(c.Surface),
		string(c.By.Executor.Kind), string(c.By.Executor.Member), c.By.Executor.Version,
		string(c.By.OnBehalfOf), s.stampNow())
	return err
}

func (s *Store) CreateTask(ctx context.Context, draft domain.Task, by domain.Provenance) (domain.Task, error) {
	if err := by.Validate(); err != nil {
		return domain.Task{}, err
	}
	if draft.Status == "" {
		draft.Status = domain.StatusTodo
	}

	var created domain.Task
	err := s.write(ctx, func(tx *sql.Tx) error {
		// Checked before the insert so the refusal names the board. Left to the foreign key, the
		// caller receives "FOREIGN KEY constraint failed (787)" — true, and useless to the agent
		// reading it, which the specification asks a tool error to avoid.
		var exists int
		if err := tx.QueryRowContext(ctx,
			`select count(*) from boards where id = ?`, string(draft.Board)).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("%w: no board %q — call list_boards to see which boards exist",
				domain.ErrNotFound, string(draft.Board))
		}

		number, err := nextTaskNumber(ctx, tx)
		if err != nil {
			return err
		}
		id, err := domain.NewTaskID(number)
		if err != nil {
			return err
		}

		now := s.stampNow()
		draft.ID = id
		draft.CreatedAt = parseStamp(now)
		draft.UpdatedAt = draft.CreatedAt
		if err := draft.Validate(); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx,
			`insert into tasks (id, number, board, title, body, status, assignee, due, created_at, updated_at)
			 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			string(id), number, string(draft.Board), draft.Title, draft.Body, string(draft.Status),
			string(draft.Assignee), draft.Due, now, now)
		if err != nil {
			return err
		}

		if err := s.record(ctx, tx, domain.Change{
			Task: id, Board: draft.Board, Kind: domain.ChangeTaskCreated, To: draft.Title, By: by,
		}); err != nil {
			return err
		}

		created = draft
		return nil
	})
	if err != nil {
		return domain.Task{}, err
	}
	return created, nil
}

// nextTaskNumber hands out the next identifier inside the caller's transaction.
//
// The counter is read and written in the same transaction as the insert, so two writers cannot mint
// the same number: whichever commits second sees the first one's value. Doing it any other way is
// how two parallel writers end up sharing a number — a mistake already paid for elsewhere.
func nextTaskNumber(ctx context.Context, tx *sql.Tx) (int64, error) {
	if _, err := tx.ExecContext(ctx,
		`insert into counters (name, value) values ('tasks', 0) on conflict (name) do nothing`); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `update counters set value = value + 1 where name = 'tasks'`); err != nil {
		return 0, err
	}
	var n int64
	if err := tx.QueryRowContext(ctx, `select value from counters where name = 'tasks'`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// edit is the shape every single-field change shares: load, refuse a no-op, write, journal.
//
// Refusing a no-op is a domain decision rather than an optimisation. An entry saying somebody moved
// a task from In review to In review is noise in a feed whose whole purpose is telling a person
// what actually changed while they were away — and an agent retrying a call would otherwise fill
// that feed by itself.
//
// The surface is passed through rather than derived, and it is domain.SurfaceNone for every kind but
// a status move: only there do two places in the interface produce the same entry, and only there is
// telling them apart a question anybody asked.
func (s *Store) edit(
	ctx context.Context,
	id domain.TaskID,
	by domain.Provenance,
	kind domain.ChangeKind,
	surface domain.Surface,
	apply func(t *domain.Task) (from, to string, changed bool),
) (domain.Task, error) {
	if err := by.Validate(); err != nil {
		return domain.Task{}, err
	}
	if !id.Valid() {
		return domain.Task{}, fmt.Errorf("%w: %q is not a task identifier", domain.ErrInvalidTask, string(id))
	}

	var result domain.Task
	err := s.write(ctx, func(tx *sql.Tx) error {
		task, err := scanTaskFor(tx.QueryRowContext(ctx, taskColumns+` from tasks where id = ?`, string(id)), id)
		if err != nil {
			return err
		}

		from, to, changed := apply(&task)
		if !changed {
			result = task
			return nil
		}
		if err := task.Validate(); err != nil {
			return err
		}

		now := s.stampNow()
		task.UpdatedAt = parseStamp(now)
		if _, err := tx.ExecContext(ctx,
			`update tasks set title = ?, body = ?, status = ?, assignee = ?, due = ?, updated_at = ? where id = ?`,
			task.Title, task.Body, string(task.Status), string(task.Assignee), task.Due, now, string(id)); err != nil {
			return err
		}

		if err := s.record(ctx, tx, domain.Change{
			Task: id, Board: task.Board, Kind: kind, From: from, To: to, Surface: surface, By: by,
		}); err != nil {
			return err
		}

		result = task
		return nil
	})
	if err != nil {
		return domain.Task{}, err
	}
	return result, nil
}

// MoveTask changes a status and records which surface asked for it.
//
// An unnamed surface is refused rather than stored as a blank. The share of moves made from the task
// screen is a number a decision waits on, and a caller that forgot to name itself would not break
// anything — it would quietly land in neither half and make the share wrong by however many calls it
// made.
func (s *Store) MoveTask(
	ctx context.Context,
	id domain.TaskID,
	to domain.Status,
	by domain.Provenance,
	from domain.Surface,
) (domain.Task, error) {
	if !to.Valid() {
		return domain.Task{}, fmt.Errorf("%w: unknown status %q", domain.ErrInvalidTask, string(to))
	}
	if !from.Named() {
		return domain.Task{}, fmt.Errorf("%w: %q", domain.ErrUnnamedSurface, string(from))
	}
	return s.edit(ctx, id, by, domain.ChangeStatusMoved, from, func(t *domain.Task) (string, string, bool) {
		was := t.Status
		t.Status = to
		return string(was), string(to), was != to
	})
}

// MoveTasks moves several tasks inside one transaction.
//
// Not a loop over MoveTask, and the difference is the whole guarantee: each call of that opens a
// transaction of its own, so a refusal halfway would leave the earlier ones committed — a partial
// outcome, under a key whose repeat must reproduce it exactly. Here the first refusal rolls the
// whole thing back, and there is nothing partial to reproduce.
//
// One journal entry per task that actually moved, written in the same transaction as the move
// itself, exactly as a single move writes it. A bulk operation that recorded one entry for the
// batch would be cheaper and would cost the product its history: the feed, the agent's cursor and
// the provenance stripe all read the journal per task.
func (s *Store) MoveTasks(
	ctx context.Context,
	ids []domain.TaskID,
	to domain.Status,
	by domain.Provenance,
	from domain.Surface,
) ([]domain.MoveResult, error) {
	if err := by.Validate(); err != nil {
		return nil, err
	}
	// The same refusal the single move makes, for the same reason: a bulk move is a third way to
	// change a status, and one that writes a blank would quietly spoil the share it feeds.
	if !from.Named() {
		return nil, fmt.Errorf("%w: %q", domain.ErrUnnamedSurface, string(from))
	}
	if !to.Valid() {
		return nil, fmt.Errorf("%w: unknown status %q", domain.ErrInvalidTask, string(to))
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no tasks were named", domain.ErrInvalidTask)
	}

	var results []domain.MoveResult
	err := s.write(ctx, func(tx *sql.Tx) error {
		// Everything that was named is read before anything is written, and the reason is the
		// refusal rather than the write: stopping at the first task that is gone would name one of
		// two, and the person would correct half the selection and meet the rest on the next try
		// (B-32). Nothing is lost by looking first — the operation is all-or-nothing anyway.
		named := make([]domain.Task, 0, len(ids))
		var missing []domain.TaskID
		for _, id := range ids {
			if !id.Valid() {
				return fmt.Errorf("%w: %q is not a task identifier", domain.ErrInvalidTask, string(id))
			}
			task, err := scanTaskFor(
				tx.QueryRowContext(ctx, taskColumns+` from tasks where id = ?`, string(id)), id)
			if errors.Is(err, domain.ErrNotFound) {
				missing = append(missing, id)
				continue
			}
			if err != nil {
				return err
			}
			named = append(named, task)
		}
		if len(missing) > 0 {
			return &domain.MissingTasks{Tasks: missing}
		}

		moved := make([]domain.MoveResult, 0, len(named))
		for _, task := range named {
			id := task.ID

			// Already there. Not an error and not an entry: the same rule a single edit applies,
			// and for the same reason — a feed telling somebody a task moved from In review to In
			// review is noise in the one place that exists to say what changed.
			if task.Status == to {
				moved = append(moved, domain.MoveResult{
					Task: id, From: task.Status, To: to, Outcome: domain.MoveUnchanged,
				})
				continue
			}

			now := s.stampNow()
			if _, err := tx.ExecContext(ctx,
				`update tasks set status = ?, updated_at = ? where id = ?`,
				string(to), now, string(id)); err != nil {
				return err
			}
			if err := s.record(ctx, tx, domain.Change{
				Task: id, Board: task.Board, Kind: domain.ChangeStatusMoved,
				From: string(task.Status), To: string(to), Surface: from, By: by,
			}); err != nil {
				return err
			}

			moved = append(moved, domain.MoveResult{
				Task: id, From: task.Status, To: to, Outcome: domain.MoveMoved,
			})
		}
		results = moved
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) AssignTask(ctx context.Context, id domain.TaskID, to domain.MemberID, by domain.Provenance) (domain.Task, error) {
	return s.edit(ctx, id, by, domain.ChangeAssigned, domain.SurfaceNone, func(t *domain.Task) (string, string, bool) {
		was := t.Assignee
		t.Assignee = to
		return string(was), string(to), was != to
	})
}

func (s *Store) SetDue(ctx context.Context, id domain.TaskID, due string, by domain.Provenance) (domain.Task, error) {
	return s.edit(ctx, id, by, domain.ChangeDueChanged, domain.SurfaceNone, func(t *domain.Task) (string, string, bool) {
		was := t.Due
		t.Due = due
		return was, due, was != due
	})
}

// Retitle changes a task's name.
//
// Separate from Rewrite rather than one call taking both, and the reason is the journal: the
// vocabulary has had `title_edited` and `body_edited` as distinct kinds since it was written, and a
// history that says "edited" where it could say which half changed is a history that makes somebody
// open the task to find out.
func (s *Store) Retitle(ctx context.Context, id domain.TaskID, title string, by domain.Provenance) (domain.Task, error) {
	return s.edit(ctx, id, by, domain.ChangeTitleEdited, domain.SurfaceNone, func(t *domain.Task) (string, string, bool) {
		was := t.Title
		t.Title = strings.TrimSpace(title)
		return was, t.Title, was != t.Title
	})
}

// Rewrite changes a task's description.
func (s *Store) Rewrite(ctx context.Context, id domain.TaskID, body string, by domain.Provenance) (domain.Task, error) {
	return s.edit(ctx, id, by, domain.ChangeBodyEdited, domain.SurfaceNone, func(t *domain.Task) (string, string, bool) {
		was := t.Body
		t.Body = strings.TrimSpace(body)
		return was, t.Body, was != t.Body
	})
}

func (s *Store) Comment(ctx context.Context, id domain.TaskID, text string, by domain.Provenance) (domain.Comment, error) {
	if err := by.Validate(); err != nil {
		return domain.Comment{}, err
	}
	if strings.TrimSpace(text) == "" {
		return domain.Comment{}, fmt.Errorf("%w: empty comment", domain.ErrInvalidTask)
	}

	var comment domain.Comment
	err := s.write(ctx, func(tx *sql.Tx) error {
		task, err := scanTaskFor(tx.QueryRowContext(ctx, taskColumns+` from tasks where id = ?`, string(id)), id)
		if err != nil {
			return err
		}

		now := s.stampNow()
		res, err := tx.ExecContext(ctx,
			`insert into comments (task, body, actor_kind, actor_member, actor_version, on_behalf_of, created_at)
			 values (?, ?, ?, ?, ?, ?, ?)`,
			string(id), text, string(by.Executor.Kind), string(by.Executor.Member),
			by.Executor.Version, string(by.OnBehalfOf), now)
		if err != nil {
			return err
		}
		commentID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		if err := s.record(ctx, tx, domain.Change{
			Task: id, Board: task.Board, Kind: domain.ChangeCommentPosted, To: text, By: by,
		}); err != nil {
			return err
		}

		comment = domain.Comment{ID: commentID, Task: id, Text: text, By: by, CreatedAt: parseStamp(now)}
		return nil
	})
	if err != nil {
		return domain.Comment{}, err
	}
	return comment, nil
}
