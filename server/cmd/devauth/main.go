// Command devauth is a fixture authorization server for local conformance runs.
//
// Not part of the product and deliberately not inside it: tacku is a resource server, and putting a
// token minter in the same binary would be a backdoor wearing a flag. This one generates a key on
// each start, serves a key set and prints one token, then keeps serving until it is killed.
//
// It exists because a conformance walk with no token proves almost nothing: every endpoint answers
// 401 and each check reports the same fact in its own words.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	addr := flag.String("addr", ":8478", "address to serve the key set on")
	issuer := flag.String("issuer", "http://localhost:8478", "issuer identifier to put in the token")
	audience := flag.String("audience", "http://localhost:8477", "the resource the token is for")
	subject := flag.String("subject", "anna-agent", "who the token is issued to")
	onBehalfOf := flag.String("on-behalf-of", "anna", "who they act for")
	pageAudience := flag.String("page-audience", "http://localhost:8477/", "the audience of tokens the redirect flow issues")
	flag.Parse()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fail(err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":           *issuer,
		"sub":           *subject,
		"aud":           *audience,
		"exp":           time.Now().Add(time.Hour).Unix(),
		"iat":           time.Now().Unix(),
		"scope":         "tasks:read tasks:write",
		"agent_version": "dev",
		"on_behalf_of":  *onBehalfOf,
	})
	token.Header["kid"] = "dev"

	signed, err := token.SignedString(key)
	if err != nil {
		fail(err)
	}

	// The token goes to stdout alone so a caller can capture it; everything else goes to stderr.
	fmt.Println(signed)

	// The redirect a page takes. The printed token above is for a walk that has no browser; this is
	// for the half of the product that does.
	pageFlow := newFlow(*issuer, *pageAudience, key)
	// /authorize is reached by leaving the page, so it needs nothing; the other three are read by
	// the page's own code and are unreadable to it without these headers. A provider that serves
	// browser clients sends them too — the stand is not being lenient, it is being ordinary.
	http.HandleFunc("/authorize", pageFlow.authorize)
	http.HandleFunc("/token", reachableFromAPage(pageFlow.token))
	http.HandleFunc("/.well-known/openid-configuration", reachableFromAPage(pageFlow.discovery))

	http.HandleFunc("GET /jwks", reachableFromAPage(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
			"kty": "RSA",
			"kid": "dev",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))

	fmt.Fprintf(os.Stderr, "devauth: serving a key set on %s for %s\n", *addr, *audience)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		fail(err)
	}
}

// reachableFromAPage lets a browser read the response. The stand has one client and no secrets to
// leak through it, so the origin is open; a real provider names the origins it knows.
func reachableFromAPage(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "devauth:", err)
	os.Exit(1)
}
