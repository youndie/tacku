package httpsrv_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
)

// fill puts more tasks in than one page holds, so that the interesting path exists at all: a full
// page followed by a short one. A list that fits in one page terminates on the first request and
// proves nothing about termination.
func (r *resource) fill(t *testing.T, count int) {
	t.Helper()
	ctx := context.Background()

	board, err := r.store.CreateBoard(ctx, "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	for i := range count {
		status := domain.StatusTodo
		if i%3 == 0 {
			status = domain.StatusInReview
		}
		if _, err := r.store.CreateTask(ctx, domain.Task{
			Board: board.ID, Title: "task", Status: status, Assignee: "anna",
		}, domain.Human("anna")); err != nil {
			t.Fatal(err)
		}
	}
}

type page struct {
	Items          []map[string]any `json:"items"`
	NextLoadAction *struct {
		URL string `json:"url"`
	} `json:"nextLoadAction"`
}

func (r *resource) page(t *testing.T, path, token string) page {
	t.Helper()

	response, body := r.get(t, path, token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %d: %s", path, response.StatusCode, body)
	}
	var decoded page
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// A walk has to end. The kit walks up to fifty pages before giving up, and the way a server fails
// this is by offering a continuation from a page that came up short.
func TestTheTaskWalkTerminatesAndSeesEverything(t *testing.T) {
	r := newResource(t)
	const total = 12
	r.fill(t, total)
	token := r.reader(t)

	seen := 0
	path := "/pages/tasks"
	for range 20 {
		current := r.page(t, path, token)
		seen += len(current.Items)
		if current.NextLoadAction == nil {
			break
		}
		if len(current.Items) == 0 {
			t.Fatal("an empty page offered a continuation, which is how a client loops for ever")
		}
		path = current.NextLoadAction.URL
	}

	if seen != total {
		t.Errorf("the walk saw %d of %d tasks", seen, total)
	}
}

// Continuation is by task number rather than by offset, and this is the case that separates them:
// something is filed while the walk is in progress. With an offset the page boundary shifts and a
// task is skipped, which is invisible to whoever is reading.
func TestAWalkDoesNotSkipWhenTheListGrowsUnderIt(t *testing.T) {
	r := newResource(t)
	r.fill(t, 12)
	token := r.reader(t)

	first := r.page(t, "/pages/tasks", token)
	if first.NextLoadAction == nil {
		t.Fatal("the first page offered no continuation, so this test has nothing to check")
	}

	boards, _ := r.store.Boards(context.Background())
	if _, err := r.store.CreateTask(context.Background(),
		domain.Task{Board: boards[0].ID, Title: "filed mid-walk", Assignee: "anna"},
		domain.Agent("anna-agent", "0.1.0", "anna")); err != nil {
		t.Fatal(err)
	}

	seen := len(first.Items)
	path := first.NextLoadAction.URL
	for range 20 {
		current := r.page(t, path, token)
		seen += len(current.Items)
		if current.NextLoadAction == nil {
			break
		}
		path = current.NextLoadAction.URL
	}

	// Twelve, or thirteen if the new one landed after the cursor. Never eleven: a task that existed
	// before the walk started must not fall through a boundary that moved.
	if seen < 12 {
		t.Errorf("the walk saw %d tasks after one was filed underneath it; something was skipped", seen)
	}
}

// One address serves the continuation and the reload with new filters, and only the client knows
// which it asked for (§8.3, §16.3).
func TestTheFilterNarrowsTheSameAddress(t *testing.T) {
	r := newResource(t)
	r.fill(t, 12)
	token := r.reader(t)

	all := r.page(t, "/pages/tasks", token)
	filtered := r.page(t, "/pages/tasks?"+url.Values{"status": {string(domain.StatusInReview)}}.Encode(), token)

	if len(filtered.Items) == 0 {
		t.Fatal("the filter matched nothing, so this test has nothing to check")
	}
	if len(filtered.Items) >= len(all.Items) && filtered.NextLoadAction != nil {
		t.Errorf("filtering returned %d items against %d unfiltered", len(filtered.Items), len(all.Items))
	}

	// The filter has to survive the continuation, or page two quietly widens the list back out.
	if filtered.NextLoadAction != nil {
		if got := filtered.NextLoadAction.URL; !contains(got, "status=in_review") {
			t.Errorf("the continuation is %q and does not carry the filter", got)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// The form and its list on one screen: the filter is an ordinary form field, the protocol having no
// separate mechanism for filtering.
func TestTheFilterIsAFormFieldOnTheSameScreen(t *testing.T) {
	r := newResource(t)
	r.fill(t, 12)

	_, body := r.get(t, "/forms/my-tasks", r.reader(t), "")

	var form struct {
		Schema struct {
			Fields []struct {
				FieldID string `json:"fieldId"`
			} `json:"fields"`
		} `json:"schema"`
		Screen map[string]any `json:"screen"`
	}
	if err := json.Unmarshal(body, &form); err != nil {
		t.Fatal(err)
	}

	if len(form.Schema.Fields) == 0 {
		t.Fatal("the screen declares no filter field, so nothing can narrow the list")
	}

	used := map[string]bool{}
	collectFieldIDs(form.Screen, used)
	for _, field := range form.Schema.Fields {
		if !used[field.FieldID] {
			t.Errorf("the schema declares %q and no input shows it", field.FieldID)
		}
	}
}
