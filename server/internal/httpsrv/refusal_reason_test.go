//go:build !debugdoor

package httpsrv_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// A refused request says which of the several possible things was wrong.
//
// Every refusal used to look identical: an expired token, one issued for another audience, and one
// signed by a provider this server has never heard of all came back as `401` with `Bearer
// realm="tacku"`. Three different mistakes, three different fixes, and nothing to tell them apart
// from a browser — which cost a day of guessing at exactly one of them.
//
// Nothing here is disclosed that the caller does not already hold: they presented the token and can
// read every claim in it themselves.
//
// Built without the instrument's door, so that what answers is the door the product ships.
func TestARefusalSaysWhatWasWrong(t *testing.T) {
	resource := newResource(t)

	t.Run("a token this server cannot read", func(t *testing.T) {
		answer := ask(t, resource.url+"/graph", "not-a-token")

		if answer.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status is %d", answer.StatusCode)
		}
		challenge := answer.Header.Get("WWW-Authenticate")
		if !strings.Contains(challenge, `error="invalid_token"`) {
			t.Fatalf("the challenge does not name the error: %q", challenge)
		}
		if !strings.Contains(challenge, "error_description=") {
			t.Fatalf("the challenge says no more than 'no': %q", challenge)
		}
	})

	t.Run("a token for another audience says so, by name", func(t *testing.T) {
		// The mistake this exists for: a client configured with the wrong resource, whose token is
		// perfectly valid and simply not addressed here.
		elsewhere := resource.as.token(t, claims{subject: "anna@tacku.team", audience: "https://somebody-elses.example/"})
		answer := ask(t, resource.url+"/graph", elsewhere)

		challenge := answer.Header.Get("WWW-Authenticate")
		if !strings.Contains(challenge, "somebody-elses.example") {
			t.Fatalf("the refusal does not say which audience the token names: %q", challenge)
		}
	})

	t.Run("the reason is in the body, where a person looks", func(t *testing.T) {
		// The header is where the specification puts it and where a library reads it. A developer
		// with a failing request in front of them sees the body, and a body saying only
		// "unauthenticated" sent somebody looking in the wrong place for an afternoon.
		answer := ask(t, resource.url+"/graph", resource.as.token(t, claims{
			subject:  "anna@tacku.team",
			audience: "https://somebody-elses.example/",
		}))

		body, err := io.ReadAll(answer.Body)
		if err != nil {
			t.Fatal(err)
		}
		var refusal struct {
			Error  string `json:"error"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(body, &refusal); err != nil {
			t.Fatalf("the refusal is not JSON: %s", body)
		}
		if !strings.Contains(refusal.Reason, "somebody-elses.example") {
			t.Fatalf("the body does not say why: %s", body)
		}
	})

	t.Run("nothing presented is told nothing beyond where it is", func(t *testing.T) {
		// An anonymous caller has no token to be told anything about, and a description here would
		// describe the absence of one.
		answer := ask(t, resource.url+"/graph", "")

		if strings.Contains(answer.Header.Get("WWW-Authenticate"), "error_description=") {
			t.Fatalf("a caller who presented nothing was given a reason: %q", answer.Header.Get("WWW-Authenticate"))
		}
	})
}

// ask makes one request, with a token when there is one, and hands back the answer unread.
func ask(t *testing.T, url, token string) *http.Response {
	t.Helper()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}
