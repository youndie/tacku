package sqlite

import (
	"context"
	"database/sql"

	"github.com/youndie/tacku/server/internal/domain"
)

func (s *Store) Comments(ctx context.Context, id domain.TaskID) ([]domain.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`select id, task, body, actor_kind, actor_member, actor_version, on_behalf_of, created_at
		 from comments where task = ? order by id`, string(id))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []domain.Comment
	for rows.Next() {
		var c domain.Comment
		var created string
		if err := rows.Scan(&c.ID, &c.Task, &c.Text,
			&c.By.Executor.Kind, &c.By.Executor.Member, &c.By.Executor.Version,
			&c.By.OnBehalfOf, &created); err != nil {
			return nil, err
		}
		c.CreatedAt = parseStamp(created)
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// Changes reads the journal after the cursor.
//
// The returned cursor advances only over the rows actually handed back, never to the end of the
// journal. A reader that asks for ten and gets ten out of forty must come back to the eleventh, not
// to the forty-first — otherwise a limit silently becomes data loss, and for an agent polling by
// cursor that loss is invisible.
func (s *Store) Changes(ctx context.Context, after domain.Cursor, limit int) ([]domain.Change, domain.Cursor, error) {
	from, err := after.Seq()
	if err != nil {
		return nil, after, err
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx,
		`select seq, task, board, kind, from_value, to_value,
		        actor_kind, actor_member, actor_version, on_behalf_of, created_at
		 from changes where seq > ? order by seq limit ?`, from, limit)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()

	changes := make([]domain.Change, 0, limit)
	last := from
	for rows.Next() {
		var c domain.Change
		var created string
		if err := rows.Scan(&c.Seq, &c.Task, &c.Board, &c.Kind, &c.From, &c.To,
			&c.By.Executor.Kind, &c.By.Executor.Member, &c.By.Executor.Version,
			&c.By.OnBehalfOf, &created); err != nil {
			return nil, after, err
		}
		c.CreatedAt = parseStamp(created)
		changes = append(changes, c)
		last = c.Seq
	}
	if err := rows.Err(); err != nil {
		return nil, after, err
	}

	return changes, domain.CursorAt(last), nil
}

// CountSince counts what stands after the cursor and how many boards it touches.
//
// Both numbers come from the database rather than from the page the screen happens to be showing,
// which is the difference between "14 changes across 3 boards" and "the 20 that fit, across every
// board you have".
func (s *Store) CountSince(ctx context.Context, after domain.Cursor) (int, int, error) {
	from, err := after.Seq()
	if err != nil {
		return 0, 0, err
	}
	var changes, boards int
	err = s.db.QueryRowContext(ctx,
		`select count(*), count(distinct board) from changes where seq > ?`, from).
		Scan(&changes, &boards)
	if err != nil {
		return 0, 0, err
	}
	return changes, boards, nil
}

// LastActors answers "who touched this last" for every task at once.
//
// The join picks the newest sequence per task inside the database rather than by reading rows and
// keeping the last one, so no page size stands between a board and the truth about its cards.
func (s *Store) LastActors(ctx context.Context) (map[domain.TaskID]domain.Provenance, error) {
	rows, err := s.db.QueryContext(ctx,
		`select c.task, c.actor_kind, c.actor_member, c.actor_version, c.on_behalf_of
		 from changes c
		 join (select task, max(seq) as seq from changes group by task) latest
		   on latest.task = c.task and latest.seq = c.seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actors := map[domain.TaskID]domain.Provenance{}
	for rows.Next() {
		var task domain.TaskID
		var by domain.Provenance
		if err := rows.Scan(&task, &by.Executor.Kind, &by.Executor.Member,
			&by.Executor.Version, &by.OnBehalfOf); err != nil {
			return nil, err
		}
		actors[task] = by
	}
	return actors, rows.Err()
}

// Latest is the cursor standing after everything written so far — what a client stores when it
// wants "from now on" rather than "from the beginning".
func (s *Store) Latest(ctx context.Context) (domain.Cursor, error) {
	var seq sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `select max(seq) from changes`).Scan(&seq); err != nil {
		return domain.Start, err
	}
	if !seq.Valid {
		return domain.Start, nil
	}
	return domain.CursorAt(seq.Int64), nil
}

func (s *Store) TaskChanges(ctx context.Context, id domain.TaskID) ([]domain.Change, error) {
	rows, err := s.db.QueryContext(ctx,
		`select seq, task, board, kind, from_value, to_value,
		        actor_kind, actor_member, actor_version, on_behalf_of, created_at
		 from changes where task = ? order by seq`, string(id))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []domain.Change
	for rows.Next() {
		var c domain.Change
		var created string
		if err := rows.Scan(&c.Seq, &c.Task, &c.Board, &c.Kind, &c.From, &c.To,
			&c.By.Executor.Kind, &c.By.Executor.Member, &c.By.Executor.Version,
			&c.By.OnBehalfOf, &created); err != nil {
			return nil, err
		}
		c.CreatedAt = parseStamp(created)
		changes = append(changes, c)
	}
	return changes, rows.Err()
}
