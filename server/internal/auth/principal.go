// Package auth turns a bearer token into the pair the domain records.
//
// The MCP specification asks a stdio server to take credentials from the environment and an HTTP
// one to be an OAuth 2.1 resource server. Both end in the same place: a principal in the request
// context. Keeping that the only channel means the tools never learn which transport they are on.
package auth

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/youndie/tacku/server/internal/domain"
)

// Scopes this resource understands.
//
// Two, not five. Every scope is a promise to enforce something, and a scope nobody checks is worse
// than no scope: it reads as a boundary in the metadata and is none.
const (
	ScopeRead  = "tasks:read"
	ScopeWrite = "tasks:write"
)

type principalKey struct{}

// Principal is who is acting, resolved once per request.
type Principal struct {
	Provenance domain.Provenance
	Scopes     []string
}

func (p Principal) Can(scope string) bool { return slices.Contains(p.Scopes, scope) }

// WithPrincipal puts the principal where the tools look for it.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom reads the principal, preferring one placed directly and falling back to the token
// the SDK's middleware verified.
//
// The fallback is what makes one code path serve both transports: over stdio nobody verified a
// token, and the principal was placed at start-up from the environment.
func PrincipalFrom(ctx context.Context) (Principal, error) {
	if p, ok := ctx.Value(principalKey{}).(Principal); ok {
		return p, nil
	}
	if info := auth.TokenInfoFromContext(ctx); info != nil {
		return FromToken(info)
	}
	return Principal{}, fmt.Errorf("auth: nobody is acting on this request")
}

// Claim names the fields this resource reads out of a token.
//
// They are ours rather than standard: nothing in OAuth says which claim carries "the person this
// agent acts for". Naming them here, in one place, keeps the assumption visible — an authorization
// server that spells them differently fails loudly at start-up rather than producing changes with
// an empty principal.
const (
	ClaimAgentVersion = "agent_version"
	ClaimOnBehalfOf   = "on_behalf_of"
)

// FromToken maps a verified token onto the domain's idea of an actor.
//
// A token that names no principal is refused rather than defaulted to the subject. An agent acting
// "for itself" is precisely the entry a reader cannot act on: something automatic happened and
// nobody wanted it.
func FromToken(info *auth.TokenInfo) (Principal, error) {
	subject := domain.MemberID(info.UserID)
	if subject == "" {
		return Principal{}, fmt.Errorf("%w: the token names no subject", auth.ErrInvalidToken)
	}

	version, _ := info.Extra[ClaimAgentVersion].(string)
	onBehalfOf, _ := info.Extra[ClaimOnBehalfOf].(string)

	var provenance domain.Provenance
	if version == "" && onBehalfOf == "" {
		provenance = domain.Human(subject)
	} else {
		if version == "" {
			return Principal{}, fmt.Errorf("%w: %s is set but %s is not — an agent must name its build",
				auth.ErrInvalidToken, ClaimOnBehalfOf, ClaimAgentVersion)
		}
		if onBehalfOf == "" {
			return Principal{}, fmt.Errorf("%w: %s is set but %s is not — an agent must name its principal",
				auth.ErrInvalidToken, ClaimAgentVersion, ClaimOnBehalfOf)
		}
		provenance = domain.Agent(subject, version, domain.MemberID(onBehalfOf))
	}

	if err := provenance.Validate(); err != nil {
		return Principal{}, fmt.Errorf("%w: %s", auth.ErrInvalidToken, err)
	}
	return Principal{Provenance: provenance, Scopes: info.Scopes}, nil
}

// ScopeString renders scopes for a WWW-Authenticate challenge.
func ScopeString(scopes ...string) string { return strings.Join(scopes, " ") }
