package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/youndie/tacku/server/internal/domain"
)

func (s *Store) Outcome(ctx context.Context, key string) (domain.Outcome, bool, error) {
	var out domain.Outcome
	err := s.db.QueryRowContext(ctx,
		`select request_hash, body from idempotency where key = ?`, key).Scan(&out.RequestHash, &out.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Outcome{}, false, nil
	}
	if err != nil {
		return domain.Outcome{}, false, err
	}
	return out, true, nil
}

func (s *Store) Remember(ctx context.Context, key string, outcome domain.Outcome) error {
	_, err := s.db.ExecContext(ctx,
		`insert into idempotency (key, request_hash, body, created_at) values (?, ?, ?, ?)`,
		key, outcome.RequestHash, outcome.Body, s.stampNow())
	return err
}

var _ domain.Attempts = (*Store)(nil)
