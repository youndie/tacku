package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

func (s *Store) Visit(ctx context.Context, member domain.MemberID) (domain.Visit, error) {
	var cursor, pending, at string
	var away int64
	err := s.db.QueryRowContext(ctx,
		`select cursor, pending, at, away from seen where member = ?`, string(member),
	).Scan(&cursor, &pending, &at, &away)
	if errors.Is(err, sql.ErrNoRows) {
		// Never here before: everything is new, which is the right answer for a first visit and the
		// only one that does not silently hide what happened before somebody joined. The zero At is
		// what tells the domain this is not a return, so no visit is advanced and no sentence about
		// a previous one is owed.
		return domain.Visit{Boundary: domain.Start, Pending: domain.Start}, nil
	}
	if err != nil {
		return domain.Visit{}, err
	}
	return domain.Visit{
		Boundary: domain.Cursor(cursor),
		Pending:  domain.Cursor(pending),
		At:       parseStamp(at),
		Away:     time.Duration(away) * time.Second,
	}, nil
}

func (s *Store) RecordVisit(ctx context.Context, member domain.MemberID, visit domain.Visit) error {
	// Both cursors are checked, and checked here rather than trusted: a fabricated one written into
	// the boundary would be read back as an error on every arrival afterwards, and the screen would
	// refuse for a reason nobody could act on.
	if _, err := visit.Boundary.Seq(); err != nil {
		return err
	}
	if _, err := visit.Pending.Seq(); err != nil {
		return err
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`insert into seen (member, cursor, pending, at, away) values (?, ?, ?, ?, ?)
			 on conflict (member) do update set
			   cursor = excluded.cursor, pending = excluded.pending,
			   at = excluded.at, away = excluded.away`,
			string(member), string(visit.Boundary), string(visit.Pending),
			visit.At.UTC().Format(stamp), int64(visit.Away/time.Second)); err != nil {
			return err
		}

		// And a row that stays. `seen` answers where a reader is now and overwrites itself doing
		// it; the question B-38 asks is about the shape of a distribution, and a distribution
		// cannot be recovered from one value per person after the fact.
		//
		// Only an arrival that began a visit — a gap of zero is the same visit continuing, and
		// counting those would fill the table with rows that measure nothing.
		if visit.Away <= 0 {
			return nil
		}
		_, err := tx.ExecContext(ctx,
			`insert into visits (member, at, away) values (?, ?, ?)`,
			string(member), visit.At.UTC().Format(stamp), int64(visit.Away/time.Second))
		return err
	})
}

// MarkSeen moves both halves of the boundary.
//
// Pending as well as the boundary, and that is not tidiness: leaving pending where it was would let
// the next arrival set the boundary back to the end of the previous visit, and the button would
// have been undone by walking away and coming back.
func (s *Store) MarkSeen(ctx context.Context, member domain.MemberID, cursor domain.Cursor) error {
	if _, err := cursor.Seq(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`insert into seen (member, cursor, pending, at, away) values (?, ?, ?, ?, 0)
		 on conflict (member) do update set
		   cursor = excluded.cursor, pending = excluded.pending,
		   at = excluded.at, away = 0`,
		string(member), string(cursor), string(cursor), s.stampNow())
	return err
}

var _ domain.Seen = (*Store)(nil)
