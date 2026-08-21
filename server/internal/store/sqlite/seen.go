package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/youndie/tacku/server/internal/domain"
)

func (s *Store) SeenAt(ctx context.Context, member domain.MemberID) (domain.Cursor, error) {
	var cursor string
	err := s.db.QueryRowContext(ctx, `select cursor from seen where member = ?`, string(member)).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		// Never here before: everything is new, which is the right answer for a first visit and the
		// only one that does not silently hide what happened before somebody joined.
		return domain.Start, nil
	}
	if err != nil {
		return domain.Start, err
	}
	return domain.Cursor(cursor), nil
}

func (s *Store) MarkSeen(ctx context.Context, member domain.MemberID, cursor domain.Cursor) error {
	if _, err := cursor.Seq(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`insert into seen (member, cursor, at) values (?, ?, ?)
		 on conflict (member) do update set cursor = excluded.cursor, at = excluded.at`,
		string(member), string(cursor), s.stampNow())
	return err
}

var _ domain.Seen = (*Store)(nil)
