package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// VerifierConfig describes the authorization server this resource trusts, and the name this
// resource answers to.
//
// The authorization server is configuration, not code. The MCP specification puts its
// implementation out of scope, and building one here would be a second product inside this one —
// see B-17. What is not optional is behaving correctly as a resource server, and that is below.
type VerifierConfig struct {
	// Issuer is the authorization server's issuer identifier. A token claiming any other issuer is
	// refused.
	Issuer string

	// Resource is this server's canonical URI, and the value a token must carry in `aud`.
	Resource string

	// JWKSURL is where the issuer publishes its signing keys.
	JWKSURL string

	// HTTPClient is used to fetch keys. Supplied by tests.
	HTTPClient *http.Client
}

// NewVerifier builds the token verifier the SDK middleware calls.
//
// The audience check is the reason this is written rather than borrowed. The specification is
// explicit — a server MUST validate that a token was issued for it, and MUST NOT accept or transit
// any other — and it is the requirement that separates a resource server from something that merely
// looks at a signature. A valid token minted for a different resource is exactly the confused
// deputy the rule exists to prevent.
func NewVerifier(config VerifierConfig) (auth.TokenVerifier, error) {
	if config.Issuer == "" || config.Resource == "" || config.JWKSURL == "" {
		return nil, fmt.Errorf("auth: issuer, resource and JWKS URL are all required")
	}

	keys := &keySet{url: config.JWKSURL, client: config.HTTPClient}
	if keys.client == nil {
		keys.client = &http.Client{Timeout: 10 * time.Second}
	}

	parser := jwt.NewParser(
		jwt.WithIssuer(config.Issuer),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"RS256"}),
	)

	return func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		claims := jwt.MapClaims{}
		parsed, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return keys.key(ctx, kid)
		})
		if err != nil || !parsed.Valid {
			return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
		}

		audiences, err := claims.GetAudience()
		if err != nil {
			return nil, fmt.Errorf("%w: unreadable audience", auth.ErrInvalidToken)
		}
		if !slices.Contains(audiences, config.Resource) {
			return nil, fmt.Errorf("%w: this token was issued for %v, not for %s",
				auth.ErrInvalidToken, []string(audiences), config.Resource)
		}

		expiry, err := claims.GetExpirationTime()
		if err != nil || expiry == nil {
			return nil, fmt.Errorf("%w: no expiry", auth.ErrInvalidToken)
		}
		subject, _ := claims.GetSubject()

		return &auth.TokenInfo{
			Scopes:     scopesOf(claims),
			Expiration: expiry.Time,
			UserID:     subject,
			Extra: map[string]any{
				ClaimAgentVersion: claims[ClaimAgentVersion],
				ClaimOnBehalfOf:   claims[ClaimOnBehalfOf],
			},
		}, nil
	}, nil
}

// scopesOf reads the space-delimited `scope` claim of RFC 8693, and tolerates the array form some
// issuers emit instead.
func scopesOf(claims jwt.MapClaims) []string {
	switch value := claims["scope"].(type) {
	case string:
		return splitScopes(value)
	case []any:
		scopes := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok {
				scopes = append(scopes, s)
			}
		}
		return scopes
	default:
		return nil
	}
}

func splitScopes(value string) []string {
	var scopes []string
	for _, s := range splitFields(value) {
		scopes = append(scopes, s)
	}
	return scopes
}

func splitFields(value string) []string {
	fields := make([]string, 0, 4)
	start := -1
	for i, r := range value {
		if r == ' ' || r == '\t' {
			if start >= 0 {
				fields = append(fields, value[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, value[start:])
	}
	return fields
}

// keySet fetches and caches the issuer's public keys.
//
// Refetched when a token names a key we do not hold, which is what happens after the issuer rotates
// — rather than on a timer, which would either refetch constantly or miss a rotation.
type keySet struct {
	url    string
	client *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

// minRefetch keeps a stream of tokens naming an unknown key from becoming a stream of requests to
// the issuer — the shape of an accidental denial of service aimed at one's own authorization server.
const minRefetch = 30 * time.Second

func (k *keySet) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	k.mu.Lock()
	key, ok := k.keys[kid]
	stale := time.Since(k.fetched) > minRefetch
	k.mu.Unlock()

	if ok {
		return key, nil
	}
	if !stale {
		return nil, fmt.Errorf("%w: unknown signing key %q", auth.ErrInvalidToken, kid)
	}

	fetched, err := k.fetch(ctx)
	if err != nil {
		return nil, err
	}

	k.mu.Lock()
	k.keys = fetched
	k.fetched = time.Now()
	key, ok = k.keys[kid]
	k.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("%w: unknown signing key %q", auth.ErrInvalidToken, kid)
	}
	return key, nil
}

func (k *keySet) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return nil, err
	}
	response, err := k.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("auth: fetching keys: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: key set answered %d", response.StatusCode)
	}

	var document struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return nil, fmt.Errorf("auth: unreadable key set: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, jwk := range document.Keys {
		if jwk.Kty != "RSA" {
			continue
		}
		modulus, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}
		exponent, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			continue
		}
		keys[jwk.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(modulus),
			E: int(new(big.Int).SetBytes(exponent).Int64()),
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("auth: the key set holds no usable RSA key")
	}
	return keys, nil
}
