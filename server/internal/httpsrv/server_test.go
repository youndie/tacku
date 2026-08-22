package httpsrv_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/httpsrv"
	"github.com/youndie/tacku/server/internal/mcpsrv"
	"github.com/youndie/tacku/server/internal/store/sqlite"
)

// authServer is the smallest thing that can stand in for an OAuth 2.1 authorization server: one
// signing key, a JWKS document and a way to mint tokens. It is test scaffolding rather than a
// product — the real one is configuration, which is the decision B-17 records.
type authServer struct {
	key    *rsa.PrivateKey
	issuer string
	http   *httptest.Server
}

func newAuthServer(t *testing.T) *authServer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	as := &authServer{key: key}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
			"kty": "RSA",
			"kid": "test",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})

	as.http = httptest.NewServer(mux)
	t.Cleanup(as.http.Close)
	as.issuer = as.http.URL
	return as
}

type claims struct {
	subject    string
	audience   string
	scopes     string
	version    string
	onBehalfOf string
	expired    bool
	issuer     string
	kid        string
}

func (as *authServer) token(t *testing.T, c claims) string {
	t.Helper()

	expiry := time.Now().Add(time.Hour)
	if c.expired {
		expiry = time.Now().Add(-time.Hour)
	}
	issuer := c.issuer
	if issuer == "" {
		issuer = as.issuer
	}

	body := jwt.MapClaims{
		"iss": issuer,
		"sub": c.subject,
		"aud": c.audience,
		"exp": expiry.Unix(),
		"iat": time.Now().Unix(),
	}
	if c.scopes != "" {
		body["scope"] = c.scopes
	}
	if c.version != "" {
		body[auth.ClaimAgentVersion] = c.version
	}
	if c.onBehalfOf != "" {
		body[auth.ClaimOnBehalfOf] = c.onBehalfOf
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, body)
	kid := c.kid
	if kid == "" {
		kid = "test"
	}
	token.Header["kid"] = kid

	signed, err := token.SignedString(as.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

type resource struct {
	url   string
	store *sqlite.Store
	as    *authServer
	clock *testClock
}

// testClock is the clock the visit boundary is measured against.
//
// A test has to be able to be away for nine hours, and waiting nine hours is not a test. Guarded by
// a mutex because the handler reads it on the server's goroutine while the test moves it on its
// own.
type testClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *testClock) pass(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

func newResource(t *testing.T) *resource {
	t.Helper()
	return newResourceWith(t, nil)
}

// newResourceWith is newResource for a test that is about a setting rather than about a screen.
// The configuration is adjusted after the defaults are in place, so a test names only what it cares
// about and inherits the rest from the server every other test runs against.
func newResourceWith(t *testing.T, adjust func(*httpsrv.Config)) *resource {
	t.Helper()

	as := newAuthServer(t)
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "tacku.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// The address the resource answers to has to be known before the handler exists, because it is
	// also the audience every token must carry. Resolved by taking the listener first and starting
	// the server afterwards — closing a placeholder to "reserve" a port does not work: the next
	// listener gets a different one, and every token then fails the audience check for a reason
	// that reads like a bug in the check.
	served := httptest.NewUnstartedServer(nil)
	base := "http://" + served.Listener.Addr().String()

	clock := &testClock{at: time.Date(2026, 8, 20, 18, 40, 0, 0, time.UTC)}

	config := httpsrv.Config{
		Deps:       mcpsrv.Deps{Store: store, Attempts: store, Version: "0.1.0"},
		Members:    store,
		Seen:       store,
		Now:        clock.now,
		SessionKey: []byte("a key of at least thirty-two bytes"),
		Verifier: auth.VerifierConfig{
			Issuer:   as.issuer,
			Resource: base + httpsrv.MCPPath,
			JWKSURL:  as.http.URL + "/jwks",
		},
	}
	if adjust != nil {
		adjust(&config)
	}

	handler, err := httpsrv.New(config)
	if err != nil {
		t.Fatal(err)
	}

	served.Config.Handler = handler
	served.Start()
	t.Cleanup(served.Close)

	return &resource{url: served.URL, store: store, as: as, clock: clock}
}

func (r *resource) audience() string { return r.url + httpsrv.MCPPath }

// bearer adds the token the way a client does, since the transport takes an http.Client rather than
// headers of its own.
type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if b.token != "" {
		clone.Header.Set("Authorization", "Bearer "+b.token)
	}
	return b.next.RoundTrip(clone)
}

func (r *resource) connect(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   r.url + httpsrv.MCPPath,
		HTTPClient: &http.Client{Transport: bearer{token: token, next: http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestMetadataIsPublishedWhereTheSpecificationSaysItIs(t *testing.T) {
	r := newResource(t)

	response, err := http.Get(r.url + httpsrv.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("metadata answered %d", response.StatusCode)
	}

	var document struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		t.Fatal(err)
	}
	if document.Resource != r.audience() {
		t.Errorf("metadata names %q as the resource, want %q", document.Resource, r.audience())
	}
	if len(document.AuthorizationServers) != 1 {
		t.Errorf("metadata names %d authorization servers", len(document.AuthorizationServers))
	}
	if !slices.Contains(document.ScopesSupported, auth.ScopeWrite) {
		t.Errorf("scopes_supported is %v", document.ScopesSupported)
	}
}

// The first step of the flow: a client with no token must be told where to go, in the header the
// specification names.
func TestAnAnonymousCallIsChallenged(t *testing.T) {
	r := newResource(t)

	response, err := http.Post(r.url+httpsrv.MCPPath, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an anonymous call answered %d, want 401", response.StatusCode)
	}
	challenge := response.Header.Get("WWW-Authenticate")
	if !strings.Contains(challenge, "resource_metadata=") {
		t.Errorf("the challenge is %q; it must point at the resource metadata", challenge)
	}
	if !strings.Contains(challenge, httpsrv.MetadataPath) {
		t.Errorf("the challenge is %q; the metadata URL is wrong", challenge)
	}
}

// The requirement that separates a resource server from something that merely checks a signature.
// A token minted by the same issuer, correctly signed and unexpired, but meant for another
// resource, is the confused deputy the rule exists to prevent.
func TestATokenForAnotherResourceIsRefused(t *testing.T) {
	r := newResource(t)

	token := r.as.token(t, claims{
		subject: "anna", audience: "https://someone-else.example/mcp", scopes: auth.ScopeRead,
	})

	if code := r.status(t, token); code != http.StatusUnauthorized {
		t.Errorf("a token for another audience was answered %d, want 401", code)
	}
}

func TestAForeignIssuerAndAnExpiredTokenAreRefused(t *testing.T) {
	r := newResource(t)

	cases := map[string]claims{
		"another issuer": {subject: "anna", audience: r.audience(), scopes: auth.ScopeRead, issuer: "https://evil.example"},
		"expired":        {subject: "anna", audience: r.audience(), scopes: auth.ScopeRead, expired: true},
		"unknown key":    {subject: "anna", audience: r.audience(), scopes: auth.ScopeRead, kid: "not-published"},
	}
	for name, c := range cases {
		if code := r.status(t, r.as.token(t, c)); code != http.StatusUnauthorized {
			t.Errorf("%s was answered %d, want 401", name, code)
		}
	}
}

func (r *resource) status(t *testing.T, token string) int {
	t.Helper()

	request, err := http.NewRequest(http.MethodPost, r.url+httpsrv.MCPPath, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

// The scope model: a caller who may not write does not see the tools that write.
func TestAReadOnlyTokenSeesNoWriteTools(t *testing.T) {
	r := newResource(t)

	session := r.connect(t, r.as.token(t, claims{
		subject: "anna-agent", audience: r.audience(), scopes: auth.ScopeRead,
		version: "0.1.0", onBehalfOf: "anna",
	}))

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}

	for _, write := range []string{"create_task", "move_task", "assign_task", "set_due", "comment_task"} {
		if slices.Contains(names, write) {
			t.Errorf("a read-only caller was offered %s", write)
		}
	}
	if !slices.Contains(names, "list_boards") {
		t.Errorf("a read-only caller was not offered list_boards; tools were %v", names)
	}
}

// The test that proves the principal travels in the request context rather than in configuration:
// this server has no fallback actor at all, so a change can only be attributed from the token.
func TestTheChangeIsAttributedToTheTokenNotToConfiguration(t *testing.T) {
	r := newResource(t)

	board, err := r.store.CreateBoard(context.Background(), "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}

	session := r.connect(t, r.as.token(t, claims{
		subject: "anna-agent", audience: r.audience(),
		scopes:  auth.ScopeString(auth.ScopeRead, auth.ScopeWrite),
		version: "0.4.2", onBehalfOf: "anna",
	}))

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_task",
		Arguments: map[string]any{"board": string(board.ID), "title": "From a token", "idempotency_key": "k1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("create_task failed: %v", result.Content)
	}

	changes, _, err := r.store.Changes(context.Background(), domain.Start, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("journal holds %d entries", len(changes))
	}

	by := changes[0].By
	if !by.ByAgent() || by.Executor.Member != "anna-agent" || by.OnBehalfOf != "anna" {
		t.Errorf("provenance is %+v; it must come from the token", by)
	}
	if by.Executor.Version != "0.4.2" {
		t.Errorf("the version recorded is %q, want the token's 0.4.2", by.Executor.Version)
	}
}

// A token that names a build but nobody to act for describes an agent acting for itself, which is
// the entry a reader cannot act on.
func TestATokenNamingNoPrincipalIsRefused(t *testing.T) {
	r := newResource(t)

	session := r.connect(t, r.as.token(t, claims{
		subject: "anna-agent", audience: r.audience(),
		scopes:  auth.ScopeString(auth.ScopeRead, auth.ScopeWrite),
		version: "0.1.0",
	}))

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_boards",
		Arguments: map[string]any{},
	})
	// A read needs no actor, so this one succeeds; the refusal has to bite on a write.
	if err != nil || result.IsError {
		t.Fatalf("a read was refused: %v %v", err, result)
	}

	written, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_task",
		Arguments: map[string]any{"board": "Sprint 24", "title": "x", "idempotency_key": "k"},
	})
	if err == nil && !written.IsError {
		t.Fatal("a write was accepted from a token that names no principal")
	}
	if written != nil {
		var text strings.Builder
		for _, c := range written.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text.WriteString(tc.Text)
			}
		}
		if !strings.Contains(text.String(), auth.ClaimOnBehalfOf) {
			t.Errorf("the refusal reads %q; it must name the claim that is missing", text.String())
		}
	}
}
