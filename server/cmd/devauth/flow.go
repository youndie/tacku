package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// The redirect a page goes through, small enough to read in one sitting.
//
// It exists because the browser client signs a person in the way the product does — a redirect to
// the provider — and there was no way to walk that path without the real one. A stand that can only
// mint a token proves the resource server verifies tokens; it proves nothing about the flow a person
// takes to get one, which is where the page's own half lives.
//
// Deliberately not an authorization server. There is no consent, no session, no registration: the
// sign-in page is a form with a name in it, because on a stand the question "who are you" has no
// wrong answer. What is real is the shape — code, verifier, exchange — since that is the part the
// page implements and the part that can be wrong.
type flow struct {
	issuer   string
	audience string
	key      *rsa.PrivateKey

	mu     sync.Mutex
	issued map[string]pending
}

type pending struct {
	subject   string
	challenge string
	redirect  string
	born      time.Time
}

func newFlow(issuer, audience string, key *rsa.PrivateKey) *flow {
	return &flow{issuer: issuer, audience: audience, key: key, issued: map[string]pending{}}
}

// authorize shows a form, and on submit hands a code back to the page.
//
// PKCE is required rather than optional: a public client has no secret, so the code is the whole
// credential and a challenge is what stops somebody who intercepted it from spending it. A stand
// that accepted a request without one would let the page ship without one too, and nothing would
// say so until production.
func (f *flow) authorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	redirect := query.Get("redirect_uri")
	state := query.Get("state")
	challenge := query.Get("code_challenge")

	if redirect == "" || challenge == "" || query.Get("code_challenge_method") != "S256" {
		http.Error(w, "devauth: this stand requires redirect_uri and an S256 code_challenge", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		f.form(w, redirect, state, challenge)
		return
	}

	subject := r.FormValue("subject")
	if subject == "" {
		http.Error(w, "devauth: name somebody", http.StatusBadRequest)
		return
	}

	code := random()
	f.mu.Lock()
	f.issued[code] = pending{subject: subject, challenge: challenge, redirect: redirect, born: time.Now()}
	f.mu.Unlock()

	back, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, "devauth: redirect_uri is not a URL", http.StatusBadRequest)
		return
	}
	parameters := back.Query()
	parameters.Set("code", code)
	if state != "" {
		parameters.Set("state", state)
	}
	back.RawQuery = parameters.Encode()

	http.Redirect(w, r, back.String(), http.StatusFound)
}

// form is the sign-in screen of a stand: a name and a button.
//
// No password. The stand's job is to be somebody, not to check that they are — a password here would
// be a second implementation of the thing the provider exists to do, tested by nobody and different
// from the real one in ways that only show in production.
func (f *flow) form(w http.ResponseWriter, redirect, state, challenge string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>devauth</title>
<style>body{background:#101114;color:#E3E5E9;font:14px/1.5 system-ui;display:flex;height:100vh;margin:0;align-items:center;justify-content:center}
form{display:flex;flex-direction:column;gap:12px;width:280px}input,button{padding:12px;border:0;font:inherit}
input{background:#23262B;color:#F2F3F5}button{background:#4869DB;color:#fff;font-weight:600}
p{color:#9AA1AC;margin:0 0 8px}</style></head><body>
<form method="post">
<p>A stand, not a provider. Whoever you say you are, you are.</p>
<input name="subject" value="anna" autofocus>
<input type="hidden" name="redirect_uri" value="%s">
<input type="hidden" name="state" value="%s">
<input type="hidden" name="code_challenge" value="%s">
<button type="submit">Sign in</button>
</form></body></html>`, html.EscapeString(redirect), html.EscapeString(state), html.EscapeString(challenge))
}

// token exchanges a code for the pair, checking the verifier the page kept to itself.
func (f *flow) token(w http.ResponseWriter, r *http.Request) {
	// The route no longer names the method — the CORS wrapper answers OPTIONS on the same path — so
	// the guard lives here. Without it a code could be spent by following a link.
	if r.Method != http.MethodPost {
		refuse(w, "invalid_request", "the token endpoint takes POST")
		return
	}

	code := r.FormValue("code")
	verifier := r.FormValue("code_verifier")

	f.mu.Lock()
	waiting, ok := f.issued[code]
	delete(f.issued, code)
	f.mu.Unlock()

	// Single use, and that is the point of deleting it above: a code that can be spent twice is a
	// code worth stealing.
	if !ok {
		refuse(w, "invalid_grant", "no such code, or it was already spent")
		return
	}
	if time.Since(waiting.born) > time.Minute {
		refuse(w, "invalid_grant", "the code is older than a minute")
		return
	}

	sum := sha256.Sum256([]byte(verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != waiting.challenge {
		refuse(w, "invalid_grant", "the verifier does not match the challenge")
		return
	}

	claims := jwt.MapClaims{
		"iss":   f.issuer,
		"sub":   waiting.subject,
		"aud":   f.audience,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"scope": "tasks:read tasks:write",
	}
	signed := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed.Header["kid"] = "dev"

	body, err := signed.SignedString(f.key)
	if err != nil {
		refuse(w, "server_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": body,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

// discovery is the document a page reads to learn where to send somebody.
func (f *flow) discovery(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                f.issuer,
		"authorization_endpoint":                f.issuer + "/authorize",
		"token_endpoint":                        f.issuer + "/token",
		"jwks_uri":                              f.issuer + "/jwks",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	})
}

func refuse(w http.ResponseWriter, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": description})
}

func random() string {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		fail(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
