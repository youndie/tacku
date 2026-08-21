package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/youndie/tacku/server/internal/domain"
)

// Sessions issues and checks the tokens the KOMPOT client carries.
//
// A second token system on one server, and it is not an oversight — it is what the two protocols
// separately require. MCP over HTTP says its server is an OAuth 2.1 resource server whose tokens
// come from an authorization server and are bound to it by audience. KOMPOT says the client
// receives a pair through `update_session` after signing in (§12.4) and sends the access half as a
// bearer (§16.7); nothing there is OAuth, and OAuth 2.1 has no grant that would let a form exchange
// a password for a token.
//
// So the earlier decision to guard both surfaces with one check was a simplification that survived
// only until the login form had to exist. Each surface now carries what its own specification asks
// for, and the two never mix: an MCP call authenticated by a session token is refused, and so is a
// screen authenticated by an OAuth token.
type Sessions struct {
	key    []byte
	life   time.Duration
	renews time.Duration
	now    func() time.Time
}

const (
	sessionIssuer = "tacku"

	// Short, because there is no revocation list. A signed token cannot be withdrawn, so the only
	// thing bounding the damage of a stolen one is how soon it stops working.
	accessLife = 15 * time.Minute

	// Long enough that a person is not asked again during a working day.
	refreshLife = 30 * 24 * time.Hour
)

func NewSessions(key []byte) (*Sessions, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("auth: the session key must be at least 32 bytes")
	}
	return &Sessions{key: key, life: accessLife, renews: refreshLife, now: time.Now}, nil
}

// Pair is what `update_session` carries.
type Pair struct {
	Access  string
	Refresh string
}

func (s *Sessions) Issue(member domain.Member) (Pair, error) {
	access, err := s.sign(member.ID, "access", s.life)
	if err != nil {
		return Pair{}, err
	}
	refresh, err := s.sign(member.ID, "refresh", s.renews)
	if err != nil {
		return Pair{}, err
	}
	return Pair{Access: access, Refresh: refresh}, nil
}

func (s *Sessions) sign(member domain.MemberID, use string, life time.Duration) (string, error) {
	now := s.now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": sessionIssuer,
		"sub": string(member),
		"use": use,
		"iat": now.Unix(),
		"exp": now.Add(life).Unix(),
	})
	return token.SignedString(s.key)
}

// Verify turns an access token back into a principal.
//
// The `use` claim is checked, so a refresh token cannot be presented as an access one. Without it
// the long-lived half would be a bearer for the short-lived half's purposes, and the fifteen minutes
// above would be decoration.
func (s *Sessions) Verify(token string) (Principal, error) {
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithIssuer(sessionIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"HS256"}),
	)

	parsed, err := parser.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return s.key, nil
	})
	if err != nil || !parsed.Valid {
		return Principal{}, fmt.Errorf("%w: %v", sdkauth.ErrInvalidToken, err)
	}
	if use, _ := claims["use"].(string); use != "access" {
		return Principal{}, fmt.Errorf("%w: this is a %q token, not an access token", sdkauth.ErrInvalidToken, use)
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return Principal{}, fmt.Errorf("%w: the token names no subject", sdkauth.ErrInvalidToken)
	}

	// A person acting for themselves. An agent never arrives this way: it has no password and no
	// screen, and the surface it uses carries OAuth.
	return Principal{
		Provenance: domain.Human(domain.MemberID(subject)),
		Scopes:     []string{ScopeRead, ScopeWrite},
	}, nil
}
