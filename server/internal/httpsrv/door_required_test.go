//go:build !debugdoor

package httpsrv_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/httpsrv"
	"github.com/youndie/tacku/server/internal/mcpsrv"
	"github.com/youndie/tacku/server/internal/store/sqlite"
)

// A server that serves screens nobody can reach must not start, and one that serves none must.
//
// The first half is the point of the rule: a release build carries no sign-in form, so a deployment
// that forgot to configure the identity provider would come up, answer, and let nobody in — a
// rollout reporting success over a product with no way into it.
//
// The second half is what the rule cost when it was written too broadly. It began as "no door, no
// start"; the conformance walk serves the agent surface alone, needs no door at all, and started
// failing with "connection refused" — which is what a refusal to start looks like from outside, and
// says nothing about why.
//
// Built only without the instrument's door, because with it compiled in there is always a way in
// and there is nothing here to refuse.
func TestADoorIsRequiredOnlyWhenAPageIsServed(t *testing.T) {
	page := t.TempDir()
	if err := os.WriteFile(filepath.Join(page, "index.html"), []byte("<title>tacku</title>"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("a page with no way in is refused", func(t *testing.T) {
		_, err := httpsrv.New(doorlessConfig(t, page))
		if err == nil {
			t.Fatal("the server started while serving a page nobody could sign in to")
		}
		if !strings.Contains(err.Error(), "nobody can sign in") {
			t.Fatalf("the refusal does not say what is wrong: %v", err)
		}
	})

	t.Run("no page means no door is needed", func(t *testing.T) {
		if _, err := httpsrv.New(doorlessConfig(t, "")); err != nil {
			t.Fatalf("a server for agents alone refused to start: %v", err)
		}
	})
}

// doorlessConfig is a server with an agent surface and no way for a person to sign in.
func doorlessConfig(t *testing.T, pageDir string) httpsrv.Config {
	t.Helper()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "tacku.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	as := newAuthServer(t)

	return httpsrv.Config{
		Deps:       mcpsrv.Deps{Store: store, Attempts: store, Version: "0.1.0"},
		Members:    store,
		Seen:       store,
		Now:        func() time.Time { return time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC) },
		SessionKey: []byte("a key of at least thirty-two bytes"),
		Verifier: auth.VerifierConfig{
			Issuer:   as.issuer,
			Resource: "http://localhost:8477" + httpsrv.MCPPath,
			JWKSURL:  as.http.URL + "/jwks",
		},
		PageDir: pageDir,
		// Page is left empty on purpose: that is the deployment being described.
	}
}
