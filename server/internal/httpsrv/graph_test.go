package httpsrv_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/youndie/tacku/server/internal/render"
)

// The test this whole item exists for.
//
// A deeplink the graph does not carry is one the client is required to ignore (§12.2), so a
// mismatch is a button that does nothing, silently, for as long as nobody happens to press it while
// paying attention. Enumerating what the server emitted found exactly that: two spellings of one
// destination, `app://board` and `app://boards`, one of which resolved to nothing.
//
// Constants alone would not have caught it — both spellings were constants of a sort, being string
// literals somebody wrote twice. What catches it is walking the trees and comparing.
func TestEveryDeeplinkTheServerEmitsResolves(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	known := map[string]bool{}
	for _, route := range render.Graph {
		known[route.Deeplink] = true
	}
	for _, native := range render.ClientNative {
		known[native] = true
	}

	// Every tree this server can serve, since a deeplink is only reachable through one.
	screens := []string{
		"/screens/catch-up",
		"/screens/board",
		"/forms/my-tasks",
		"/forms/new-task",
		"/forms/new-board",
		"/forms/sign-in",
	}

	emitted := map[string][]string{}
	collect := func(from *resource, token, path, label string) {
		response, body := from.get(t, path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s answered %d", path, response.StatusCode)
		}
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Fatal(err)
		}
		for _, link := range collectDeeplinks(tree) {
			emitted[link] = append(emitted[link], label)
		}
	}

	for _, path := range screens {
		collect(r, token, path, path)
	}

	// And again against an empty workspace, because the empty states are trees of their own and
	// carry their own destinations. Without this the check missed `app://new-board` entirely: it is
	// emitted only when there is no board, and every other test seeds one — which a mutation
	// removing that route from the graph proved by passing.
	empty := newResource(t)
	emptyToken := empty.reader(t)
	for _, path := range []string{"/screens/board", "/forms/my-tasks"} {
		collect(empty, emptyToken, path, path+" (empty)")
	}

	if len(emitted) == 0 {
		t.Fatal("no deeplink was found in any tree, so this check has nothing to look at")
	}

	for link, where := range emitted {
		if !known[link] {
			t.Errorf("%s is emitted by %v and is neither a route of the graph nor known to the client: pressing it does nothing", link, where)
		}
	}
}

// And the other direction: a route promising a screen that does not answer sends a client looking
// for something that was never there.
func TestEveryRouteOfTheGraphAnswers(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	response, body := r.get(t, "/graph", token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the graph answered %d", response.StatusCode)
	}

	var graph struct {
		Routes []render.Route `json:"routes"`
	}
	if err := json.Unmarshal(body, &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Routes) < 2 {
		t.Fatalf("the graph holds %d routes, which is too few to be this product", len(graph.Routes))
	}

	for _, route := range graph.Routes {
		got, _ := r.get(t, route.Endpoint, token, "")
		if got.StatusCode != http.StatusOK {
			t.Errorf("the graph promises %s at %s, which answered %d", route.Deeplink, route.Endpoint, got.StatusCode)
		}
	}
}

// A screen needing client code of its own stays out of the graph (§12.1). Signing in is the sharpest
// case and the reason is structural rather than editorial: fetching the graph needs a session, and
// somebody who needs to sign in has none.
func TestSigningInIsNotPromisedByTheGraph(t *testing.T) {
	for _, route := range render.Graph {
		if route.Deeplink == render.LinkSignIn {
			t.Error("the graph carries the sign-in screen, which nobody who needs it could fetch it to learn about")
		}
	}
	if !slices.Contains(render.ClientNative, render.LinkSignIn) {
		t.Error("signing in is neither in the graph nor listed as known to the client, so it is a dead end")
	}
}

func collectDeeplinks(node any) []string {
	var found []string
	switch value := node.(type) {
	case map[string]any:
		if value["type"] == "navigate" {
			if link, ok := value["deeplink"].(string); ok {
				found = append(found, link)
			}
		}
		for _, child := range value {
			found = append(found, collectDeeplinks(child)...)
		}
	case []any:
		for _, child := range value {
			found = append(found, collectDeeplinks(child)...)
		}
	}
	return found
}
