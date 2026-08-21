package domain

import (
	"context"
	"errors"
)

// Member is a person of the workspace. An agent acts on behalf of one and has no account of its own
// here: over stdio it is named by the environment, over HTTP by the token its authorization server
// issued.
type Member struct {
	ID    MemberID
	Email string
	Name  string
}

// ErrBadCredentials is deliberately one error for both halves. Answering "no such email" separately
// tells anybody who asks which addresses are registered.
var ErrBadCredentials = errors.New("domain: the email and password do not match")

// Members is the half of storage that knows who people are.
type Members interface {
	Member(ctx context.Context, id MemberID) (Member, error)

	// Authenticate returns the member when the password is right, and ErrBadCredentials otherwise.
	Authenticate(ctx context.Context, email, password string) (Member, error)

	// AddMember creates one. Used by seeding and, later, by whatever invites people.
	AddMember(ctx context.Context, id MemberID, email, name, password string) (Member, error)
}
