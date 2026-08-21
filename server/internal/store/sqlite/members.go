package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/youndie/tacku/server/internal/domain"
)

func (s *Store) AddMember(ctx context.Context, id domain.MemberID, email, name, password string) (domain.Member, error) {
	if id == "" || email == "" {
		return domain.Member{}, fmt.Errorf("%w: a member needs an identifier and an email", domain.ErrInvalidTask)
	}
	if len(password) < 8 {
		return domain.Member{}, fmt.Errorf("%w: a password shorter than eight characters", domain.ErrInvalidTask)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.Member{}, err
	}

	member := domain.Member{ID: id, Email: strings.ToLower(email), Name: name}
	err = s.write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`insert into members (id, email, name, password, created_at) values (?, ?, ?, ?, ?)`,
			string(id), member.Email, name, string(hashed), s.stampNow())
		return err
	})
	if err != nil {
		return domain.Member{}, err
	}
	return member, nil
}

func (s *Store) Member(ctx context.Context, id domain.MemberID) (domain.Member, error) {
	var member domain.Member
	err := s.db.QueryRowContext(ctx,
		`select id, email, name from members where id = ?`, string(id)).
		Scan(&member.ID, &member.Email, &member.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Member{}, fmt.Errorf("%w: no member %q", domain.ErrNotFound, string(id))
	}
	return member, err
}

// Authenticate compares the password against the stored hash.
//
// The comparison runs even when no such email exists, against a hash of nothing. Returning early
// would answer a stranger in a few microseconds and a member in a few hundred, and the difference is
// readable over a network: an attacker learns which addresses are registered without guessing a
// single password.
func (s *Store) Authenticate(ctx context.Context, email, password string) (domain.Member, error) {
	var member domain.Member
	var hashed string
	err := s.db.QueryRowContext(ctx,
		`select id, email, name, password from members where email = ?`, strings.ToLower(strings.TrimSpace(email))).
		Scan(&member.ID, &member.Email, &member.Name, &hashed)

	if errors.Is(err, sql.ErrNoRows) {
		hashed = absentMemberHash
	} else if err != nil {
		return domain.Member{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) != nil {
		return domain.Member{}, domain.ErrBadCredentials
	}
	if member.ID == "" {
		return domain.Member{}, domain.ErrBadCredentials
	}
	return member, nil
}

// absentMemberHash is a real bcrypt hash of a value nothing will match, so that the comparison for
// an unknown email costs what a known one costs.
const absentMemberHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

var _ domain.Members = (*Store)(nil)
