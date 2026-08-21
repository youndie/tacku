package httpsrv_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/domain"
)

func (r *resource) get(t *testing.T, path, token, ifNoneMatch string) (*http.Response, []byte) {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, r.url+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response, body
}

func (r *resource) reader(t *testing.T) string {
	t.Helper()
	return r.as.token(t, claims{
		subject: "anna-agent", audience: r.audience(),
		scopes:  auth.ScopeString(auth.ScopeRead, auth.ScopeWrite),
		version: "0.1.0", onBehalfOf: "anna",
	})
}

func (r *resource) seed(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	board, err := r.store.CreateBoard(ctx, "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	task, err := r.store.CreateTask(ctx,
		domain.Task{Board: board.ID, Title: "Fix login redirect loop"},
		domain.Agent("anna-agent", "0.1.0", "anna"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.store.MoveTask(ctx, task.ID, domain.StatusInReview, domain.Human("ivan")); err != nil {
		t.Fatal(err)
	}
}

func TestTheScreenIsRefusedWithoutAToken(t *testing.T) {
	r := newResource(t)

	response, _ := r.get(t, "/screens/catch-up", "", "")
	if response.StatusCode != http.StatusUnauthorized {
		t.Errorf("an anonymous read answered %d, want 401", response.StatusCode)
	}
	if response.Header.Get("WWW-Authenticate") == "" {
		t.Error("the refusal carries no challenge, so a client cannot find the authorization server")
	}
}

func TestTheScreenIsATreeWithUniqueIdentifiers(t *testing.T) {
	r := newResource(t)
	r.seed(t)

	response, body := r.get(t, "/screens/catch-up", r.reader(t), "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the screen answered %d: %s", response.StatusCode, body)
	}

	var tree map[string]any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}
	if tree["type"] != "row" {
		t.Errorf("the root is %v; the root of a screen carries its discriminator like any node", tree["type"])
	}

	seen := map[string]int{}
	walk(tree, seen)
	for id, count := range seen {
		if count > 1 {
			t.Errorf("identifier %q appears %d times; a live update addressed to it has nowhere to land", id, count)
		}
		if id == "" {
			t.Error("a node carries an empty identifier")
		}
	}
	if len(seen) < 10 {
		t.Errorf("the tree holds %d identified nodes, which is too few to be the screen", len(seen))
	}
}

// walk descends through the fields that hold components, and only those.
//
// Not through everything carrying a "type": a modifier has one too, and the discriminator alone
// cannot tell {"type":"padding"} from {"type":"text"}. A client parses by position rather than by
// shape, and a walker that does otherwise counts thirty-three modifiers as nodes without
// identifiers — which is what the first version of this test did.
func walk(node any, seen map[string]int) {
	value, ok := node.(map[string]any)
	if !ok {
		return
	}

	id, hasID := value["id"].(string)
	if !hasID || id == "" {
		seen[""]++
	} else {
		seen[id]++
	}

	for _, field := range []string{"children", "initialItems"} {
		if children, ok := value[field].([]any); ok {
			for _, child := range children {
				walk(child, seen)
			}
		}
	}
	for _, field := range []string{"emptyState", "content"} {
		if child, ok := value[field].(map[string]any); ok {
			walk(child, seen)
		}
	}
}

// Conditional delivery works only if the body repeats byte for byte, which is a property of the
// identifiers being derived from the data rather than from the walk. This test would fail on a
// counter, and it is the reason there is no counter.
func TestTheSameScreenComesBackWithTheSameTagAnd304(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	first, body := r.get(t, "/screens/catch-up", token, "")
	tag := first.Header.Get("ETag")
	if tag == "" {
		t.Fatal("the screen carries no ETag, so a client has nothing to revalidate with")
	}

	again, _ := r.get(t, "/screens/catch-up", token, "")
	if again.Header.Get("ETag") != tag {
		t.Errorf("two requests for unchanged data produced %q and %q; a 304 would never happen",
			tag, again.Header.Get("ETag"))
	}

	cached, empty := r.get(t, "/screens/catch-up", token, tag)
	if cached.StatusCode != http.StatusNotModified {
		t.Errorf("revalidation answered %d, want 304", cached.StatusCode)
	}
	if len(empty) != 0 {
		t.Errorf("a 304 carried %d bytes of body", len(empty))
	}
	if len(body) == 0 {
		t.Error("the first response carried no body")
	}
}

func TestAChangedScreenChangesItsTag(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	before, _ := r.get(t, "/screens/catch-up", token, "")

	board, _ := r.store.Boards(context.Background())
	if _, err := r.store.CreateTask(context.Background(),
		domain.Task{Board: board[0].ID, Title: "Something new"}, domain.Human("ivan")); err != nil {
		t.Fatal(err)
	}

	after, _ := r.get(t, "/screens/catch-up", token, "")
	if after.Header.Get("ETag") == before.Header.Get("ETag") {
		t.Error("the screen changed and its tag did not; a client would keep showing the old one")
	}
}

// A walk of pages must end. A next address handed out for a short page loops a client for ever.
func TestThePageWalkTerminates(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	response, body := r.get(t, "/pages/changes", token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the page answered %d: %s", response.StatusCode, body)
	}

	var page struct {
		Items          []any `json:"items"`
		NextLoadAction *struct {
			URL string `json:"url"`
		} `json:"nextLoadAction"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	if page.NextLoadAction != nil {
		t.Errorf("a page holding %d of a possible 20 entries still offered a next address", len(page.Items))
	}
}

func TestTheGraphOnlyPromisesScreensThatWork(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	_, body := r.get(t, "/graph", token, "")
	var graph struct {
		Routes []struct {
			Deeplink string `json:"deeplink"`
			Endpoint string `json:"endpoint"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(body, &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Routes) == 0 {
		t.Fatal("the graph is empty")
	}

	for _, route := range graph.Routes {
		response, _ := r.get(t, route.Endpoint, token, "")
		if response.StatusCode != http.StatusOK {
			t.Errorf("the graph promises %s at %s, which answered %d",
				route.Deeplink, route.Endpoint, response.StatusCode)
		}
	}
}
