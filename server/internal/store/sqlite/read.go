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
