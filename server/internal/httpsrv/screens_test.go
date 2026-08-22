package httpsrv_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/httpsrv"
	"github.com/youndie/tacku/server/internal/render"
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

// reader signs in the way a person does.
//
// The KOMPOT surface takes the pair this server issues through the sign-in form, not a token from
// the authorization server: two protocols, two token systems, and neither accepts the other's — see
// auth.Sessions.
func (r *resource) reader(t *testing.T) string {
	t.Helper()

	const password = "a-good-password"
	if _, err := r.store.AddMember(context.Background(), "anna", "anna@tacku.team", "Anna", password); err != nil {
		t.Fatal(err)
	}

	response := r.request(t, http.MethodPost, httpsrv.LoginPath, "",
		`{"formId":"sign_in","fieldId":"","values":{`+
			`"email":{"type":"text_value","text":"anna@tacku.team"},`+
			`"password":{"type":"text_value","text":"`+password+`"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("signing in answered %d", response.StatusCode)
	}

	var session struct {
		Type        string `json:"type"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	// The answer to a submit is an action the client runs through the same chain as any other
	// intent — §16.4 — and for a sign-in that action is update_session.
	if session.Type != "update_session" {
		t.Fatalf("signing in answered %q", session.Type)
	}
	return session.AccessToken
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

func (r *resource) post(t *testing.T, path, token, key, body string) *http.Response {
	t.Helper()
	return r.request(t, http.MethodPost, path, token, body, key)
}

func (r *resource) request(t *testing.T, method, path, token, body string, key ...string) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, r.url+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if len(key) > 0 && key[0] != "" {
		request.Header.Set("Idempotency-Key", key[0])
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// The headline is a measurement, so it has to measure.
//
// "14 changes across 3 boards" is the first sentence of the product's main screen, and both numbers
// used to come from whatever was already in hand: the length of the page being rendered, and the
// number of boards in the workspace. Both agree with the screen beside them and neither answers the
// question asked, which is the shape of a wrong number nobody catches.
func TestTheCatchUpHeadlineCountsEverythingWaitingAndOnlyTheBoardsTouched(t *testing.T) {
	r := newResource(t)
	ctx := context.Background()

	touched, err := r.store.CreateBoard(ctx, "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	alsoTouched, err := r.store.CreateBoard(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	// A board nobody has touched, so that counting boards and counting boards-with-changes give
	// different answers. Without it this check passes either way.
	if _, err := r.store.CreateBoard(ctx, "Someday"); err != nil {
		t.Fatal(err)
	}

	// More than one page, so that counting the page and counting the journal differ too.
	const total = 25
	for i := range total {
		board := touched.ID
		if i%5 == 0 {
			board = alsoTouched.ID
		}
		if _, err := r.store.CreateTask(ctx,
			domain.Task{Board: board, Title: "task", Assignee: "anna"},
			domain.Human("anna")); err != nil {
			t.Fatal(err)
		}
	}

	_, body := r.get(t, "/screens/catch-up", r.reader(t), "")
	want := "25 changes across 2 boards"
	if !strings.Contains(string(body), want) {
		t.Errorf("the catch-up headline does not say %q; it says %q", want, headline(t, body))
	}
}

// headline digs out the summary line so a failure names what was actually rendered.
func headline(t *testing.T, body []byte) string {
	t.Helper()

	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}
	found := ""
	var walk func(node any)
	walk = func(node any) {
		value, ok := node.(map[string]any)
		if !ok {
			return
		}
		if value["id"] == "feed-count" {
			found, _ = value["text"].(string)
		}
		for _, field := range []string{"children", "initialItems"} {
			if children, ok := value[field].([]any); ok {
				for _, child := range children {
					walk(child)
				}
			}
		}
		if child, ok := value["screen"].(map[string]any); ok {
			walk(child)
		}
	}
	walk(tree)
	return found
}

// A catch-up screen you cannot leave is a report, not a screen.
//
// The AC of the item is that a person moves from the feed into the task, and the protocol makes
// this less obvious than it sounds: no modifier makes a node tappable and a table row is a list of
// strings, so the only component carrying an action is a button. The way out is therefore a button
// on every row, and this checks that every row has one pointing at its own task.
func TestEveryFeedRowLeadsToItsOwnTask(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)

	_, body := r.get(t, "/screens/catch-up", r.reader(t), "")

	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}

	rows := 0
	var walk func(node any)
	walk = func(node any) {
		value, ok := node.(map[string]any)
		if !ok {
			return
		}
		id, _ := value["id"].(string)
		if feedRow.MatchString(id) {
			rows++
			// Read off the row itself rather than hunted for among its children: as of kompot 0.15
			// the container carries the action, and a check that keeps looking inside would go on
			// passing if the row stopped being the target and a stray button remained.
			task := deeplinkOf(value)
			if task == "" {
				t.Errorf("row %q offers no way into a task", id)
			} else if !strings.HasPrefix(task, render.LinkTask) {
				t.Errorf("row %q navigates to %q, which is not a task", id, task)
			}
		}
		for _, field := range []string{"children", "initialItems"} {
			if children, ok := value[field].([]any); ok {
				for _, child := range children {
					walk(child)
				}
			}
		}
	}
	walk(tree)

	if rows == 0 {
		t.Fatal("the feed had no rows, so this check had nothing to look at")
	}
}

// Every list of tasks opens what it lists.
//
// Three surfaces show tasks and until kompot 0.15 none of them could be opened: the vocabulary had
// no action on a container, and app://task/<id> existed only as the answer to a submitted form. So
// the product had a screen per task that a person could reach by filing something and in no other
// way.
func TestEveryListOpensWhatItLists(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	for _, surface := range []struct {
		path string
		row  *regexp.Regexp
	}{
		{"/screens/board", regexp.MustCompile(`^card-TAC-\d+-open$`)},
		{"/forms/my-tasks", regexp.MustCompile(`^row-TAC-\d+$`)},
		{"/screens/catch-up", feedRow},
	} {
		_, body := r.get(t, surface.path, token, "")
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Fatal(err)
		}

		found := 0
		var walk func(node any)
		walk = func(node any) {
			value, ok := node.(map[string]any)
			if !ok {
				return
			}
			if id, _ := value["id"].(string); surface.row.MatchString(id) {
				found++
				if deeplink := deeplinkOf(value); !strings.HasPrefix(deeplink, render.LinkTask) {
					t.Errorf("%s: %q leads to %q, which is not a task", surface.path, id, deeplink)
				}
			}
			for _, field := range []string{"children", "initialItems"} {
				if children, ok := value[field].([]any); ok {
					for _, child := range children {
						walk(child)
					}
				}
			}
			if child, ok := value["screen"].(map[string]any); ok {
				walk(child)
			}
		}
		walk(tree)

		if found == 0 {
			t.Errorf("%s: no row matched %s, so this surface was not checked at all", surface.path, surface.row)
		}
	}
}

// feedRow matches a row of the feed and nothing inside it.
var feedRow = regexp.MustCompile(`^change-\d+$`)

func deeplinkOf(node map[string]any) string {
	action, ok := node["action"].(map[string]any)
	if !ok {
		return ""
	}
	deeplink, _ := action["deeplink"].(string)
	return deeplink
}
