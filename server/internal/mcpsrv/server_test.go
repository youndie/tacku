package mcpsrv_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/youndie/tacku/server/internal/docsboard"
	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/mcpsrv"
	"github.com/youndie/tacku/server/internal/store/sqlite"
)

const (
	anna  = domain.MemberID("anna")
	robot = domain.MemberID("anna-agent")
)

type harness struct {
	session *mcp.ClientSession
	store   *sqlite.Store
	ctx     context.Context
}

func start(t *testing.T) harness { return startWith(t, nil) }

func startWith(t *testing.T, docs *docsboard.Source) harness {
	t.Helper()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "tacku.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	fallback := domain.Agent(robot, "0.1.0", anna)
	server, err := mcpsrv.New(mcpsrv.Deps{
		Store: store, Attempts: store, Docs: docs,
		Version: "0.1.0", Fallback: &fallback,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return harness{session: session, store: store, ctx: ctx}
}

func (h harness) call(t *testing.T, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	res, err := h.session.CallTool(h.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s reported an error: %v", name, res.Content)
	}
	if out != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

func (h harness) callExpectingFailure(t *testing.T, name string, args map[string]any) string {
	t.Helper()
	res, err := h.session.CallTool(h.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return err.Error()
	}
	if !res.IsError {
		t.Fatalf("%s was expected to fail and did not", name)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return text.String()
}

func TestEveryToolIsListedWithBothSchemas(t *testing.T) {
	h := start(t)

	tools, err := h.session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"assign_task", "changes_since", "comment_task", "create_task",
		"get_task", "list_boards", "list_tasks", "move_task", "set_due",
	}
	got := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
		if tool.InputSchema == nil {
			t.Errorf("%s has no inputSchema", tool.Name)
		}
		// The output schema is what lets a client validate structuredContent. It is generated
		// from the Go result type, so a tool without one is a tool whose result type is wrong.
		if tool.OutputSchema == nil {
			t.Errorf("%s has no outputSchema", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("%s has no description; the description is what the model reads to choose it", tool.Name)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("tools are %v, want %v", got, want)
	}
}

func TestAgentWritesCarryBothActors(t *testing.T) {
	h := start(t)
	board := h.board(t)

	var created struct {
		Task struct{ ID, Title, Status string } `json:"task"`
	}
	h.call(t, "create_task", map[string]any{
		"board": board, "title": "Rotate the SSO signing keys", "idempotency_key": "k1",
	}, &created)

	if created.Task.ID == "" || created.Task.Status != "todo" {
		t.Fatalf("created %+v", created.Task)
	}

	var changes struct {
		Changes []struct {
			Task string
			Kind string
			By   struct {
				Kind       string `json:"kind"`
				Member     string `json:"member"`
				Version    string `json:"version"`
				OnBehalfOf string `json:"on_behalf_of"`
			}
		} `json:"changes"`
		Cursor string `json:"cursor"`
	}
	h.call(t, "changes_since", map[string]any{}, &changes)

	if len(changes.Changes) != 1 {
		t.Fatalf("journal has %d entries, want 1", len(changes.Changes))
	}
	by := changes.Changes[0].By
	if by.Kind != "agent" || by.Member != string(robot) || by.OnBehalfOf != string(anna) || by.Version != "0.1.0" {
		t.Errorf("provenance came back as %+v", by)
	}
}

// The point of the cursor: an agent asks what moved, not for everything.
func TestChangesSinceReturnsOnlyWhatFollowed(t *testing.T) {
	h := start(t)
	board := h.board(t)

	h.call(t, "create_task", map[string]any{"board": board, "title": "one", "idempotency_key": "k1"}, nil)

	var first struct {
		Cursor  string `json:"cursor"`
		Changes []any  `json:"changes"`
	}
	h.call(t, "changes_since", map[string]any{}, &first)
	if len(first.Changes) != 1 {
		t.Fatalf("first read saw %d changes", len(first.Changes))
	}

	h.call(t, "create_task", map[string]any{"board": board, "title": "two", "idempotency_key": "k2"}, nil)

	var second struct {
		Cursor  string                  `json:"cursor"`
		Changes []struct{ Kind string } `json:"changes"`
	}
	h.call(t, "changes_since", map[string]any{"cursor": first.Cursor}, &second)
	if len(second.Changes) != 1 {
		t.Fatalf("reading from the cursor saw %d changes, want the one that followed", len(second.Changes))
	}

	var third struct {
		Changes []any `json:"changes"`
	}
	h.call(t, "changes_since", map[string]any{"cursor": second.Cursor}, &third)
	if len(third.Changes) != 0 {
		t.Errorf("nothing happened, yet the cursor returned %d entries", len(third.Changes))
	}
}

func TestARepeatedCallWithTheSameKeyCreatesOneTask(t *testing.T) {
	h := start(t)
	board := h.board(t)

	args := map[string]any{"board": board, "title": "Audit the session cookie flags", "idempotency_key": "attempt-1"}

	var first, second struct {
		Task struct{ ID string } `json:"task"`
	}
	h.call(t, "create_task", args, &first)
	h.call(t, "create_task", args, &second)

	if first.Task.ID != second.Task.ID {
		t.Errorf("the repeat produced %q instead of replaying %q", second.Task.ID, first.Task.ID)
	}

	var tasks struct {
		Tasks []any `json:"tasks"`
	}
	h.call(t, "list_tasks", map[string]any{"board": board}, &tasks)
	if len(tasks.Tasks) != 1 {
		t.Errorf("the board holds %d tasks after one create repeated twice", len(tasks.Tasks))
	}
}

func TestTheSameKeyWithADifferentRequestIsRefused(t *testing.T) {
	h := start(t)
	board := h.board(t)

	h.call(t, "create_task", map[string]any{"board": board, "title": "one", "idempotency_key": "shared"}, nil)
	message := h.callExpectingFailure(t, "create_task",
		map[string]any{"board": board, "title": "something else", "idempotency_key": "shared"})

	if !strings.Contains(message, "different request") {
		t.Errorf("the refusal reads %q; it must say the key was reused for something else", message)
	}
}

// Without a key an agent's retry is indistinguishable from a second intent, so the key is required
// rather than optional, and the refusal has to name the field the model must add.
//
// The refusal arrives from the SDK's own validation of the generated inputSchema, before any of our
// code runs: a field without `omitempty` lands in `required`. The empty-key check inside idem.Once
// is therefore unreachable from this surface — and not dead, because the HTTP submit will reach it
// with a header that is simply absent.
func TestAWriteWithoutAKeyIsRefused(t *testing.T) {
	h := start(t)
	board := h.board(t)

	message := h.callExpectingFailure(t, "create_task", map[string]any{"board": board, "title": "no key"})
	if !strings.Contains(message, "idempotency_key") {
		t.Errorf("the refusal reads %q; it must name the missing field", message)
	}
}

// A failed attempt must not be remembered: a request corrected after a refusal would otherwise keep
// receiving the refusal under the key it was refused with.
func TestAFailedAttemptDoesNotPoisonItsKey(t *testing.T) {
	h := start(t)
	board := h.board(t)

	h.callExpectingFailure(t, "create_task", map[string]any{"board": board, "title": "", "idempotency_key": "retry"})

	var fixed struct {
		Task struct{ ID string } `json:"task"`
	}
	h.call(t, "create_task", map[string]any{"board": board, "title": "now with a title", "idempotency_key": "retry"}, &fixed)
	if fixed.Task.ID == "" {
		t.Error("the corrected request was refused under the key its predecessor failed with")
	}
}

func (h harness) board(t *testing.T) string {
	t.Helper()
	b, err := h.store.CreateBoard(h.ctx, "Sprint 24")
	if err != nil {
		t.Fatal(err)
	}
	return string(b.ID)
}

// The message a model receives when it names a board that does not exist.
//
// Found by running the real binary rather than by a test: every test above creates its board first,
// so the path was never taken, and the caller received "FOREIGN KEY constraint failed (787)" — true
// and useless. The specification asks a tool error to be something a model can self-correct from.
func TestAnUnknownBoardIsRefusedInWords(t *testing.T) {
	h := start(t)

	message := h.callExpectingFailure(t, "create_task",
		map[string]any{"board": "Sprint 99", "title": "x", "idempotency_key": "k"})

	if !strings.Contains(message, "Sprint 99") {
		t.Errorf("the refusal reads %q; it must name the board that is missing", message)
	}
	if !strings.Contains(message, "list_boards") {
		t.Errorf("the refusal reads %q; it must point at the call that answers the question", message)
	}
	if strings.Contains(message, "FOREIGN KEY") || strings.Contains(message, "787") {
		t.Errorf("a storage error reached the agent: %q", message)
	}
}

func TestAnUnknownTaskIsRefusedInWords(t *testing.T) {
	h := start(t)

	message := h.callExpectingFailure(t, "move_task",
		map[string]any{"task": "TAC-9999", "status": "done", "idempotency_key": "k"})

	if !strings.Contains(message, "TAC-9999") {
		t.Errorf("the refusal reads %q; it must name the task", message)
	}
}

// A tool call names a surface of its own, and it is not one of the two people use.
//
// The share the board's mechanics depend on is counted over the human surfaces, and agent moves are
// deliberately out of the denominator: an agent has no screen to have opened. Recorded as its own
// value rather than left blank, so that "moved by a program" and "nobody wrote down where this came
// from" cannot be confused for one another when the counting is finally done.
func TestAToolMoveIsRecordedAsTheAgentSurface(t *testing.T) {
	h := start(t)
	board := h.board(t)

	var created struct {
		Task struct{ ID string } `json:"task"`
	}
	h.call(t, "create_task", map[string]any{
		"board": board, "title": "Retire the legacy webhook", "idempotency_key": "k1",
	}, &created)
	h.call(t, "move_task", map[string]any{
		"task": created.Task.ID, "status": "in_progress", "idempotency_key": "k2",
	}, nil)

	changes, err := h.store.TaskChanges(h.ctx, domain.TaskID(created.Task.ID))
	if err != nil {
		t.Fatal(err)
	}
	moves := 0
	for _, change := range changes {
		if change.Kind != domain.ChangeStatusMoved {
			continue
		}
		moves++
		if change.Surface != domain.SurfaceAgent {
			t.Errorf("a move made by a tool call is recorded as %q, want %q",
				change.Surface, domain.SurfaceAgent)
		}
	}
	if moves != 1 {
		t.Fatalf("the task's history holds %d status moves, want the one just made", moves)
	}
}

// The board a person sees and an agent does not is the one place this product's claim is checkable,
// and it was false: nine tools, and not one of them knew the docs board existed.
func TestTheAgentSeesTheDocumentedBacklog(t *testing.T) {
	h := startWith(t, docsFixture(t))

	var listed struct {
		Source string `json:"source"`
		ReadAt string `json:"readAt"`
		Items  []struct {
			ID     string `json:"id"`
			Stage  string `json:"stage"`
			Status string `json:"status"`
		} `json:"items"`
	}
	h.call(t, "list_docs_items", map[string]any{"open": true}, &listed)

	if len(listed.Items) != 1 || listed.Items[0].ID != "B-02" {
		t.Fatalf("открытых задач %d: %+v", len(listed.Items), listed.Items)
	}
	if listed.Items[0].Stage != "stage-one" {
		t.Errorf("этап не приехал: %q", listed.Items[0].Stage)
	}
	if listed.ReadAt == "" || listed.Source == "" {
		t.Errorf("ответ не называет ни источник, ни время чтения: %+v", listed)
	}

	var item struct {
		Body string `json:"body"`
		Path string `json:"path"`
	}
	h.call(t, "get_docs_item", map[string]any{"id": "B-02"}, &item)
	if !strings.Contains(item.Body, "never delivered") {
		t.Errorf("текст задачи не приехал: %q", item.Body)
	}
	if item.Path == "" {
		t.Error("путь к файлу не назван — его нечего процитировать человеку с чекаутом")
	}

	if refusal := h.callExpectingFailure(t, "get_docs_item", map[string]any{"id": "B-999"}); !strings.Contains(refusal, "B-999") {
		t.Errorf("отказ не назвал, какого идентификатора нет: %q", refusal)
	}
}

// Without a source the tools are absent rather than refusing: a model that cannot see a tool does
// not spend a turn discovering that it may not use it.
func TestWithoutASourceTheAgentIsNotOfferedTheTools(t *testing.T) {
	h := start(t)

	listed, err := h.session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range listed.Tools {
		if strings.Contains(tool.Name, "docs") {
			t.Errorf("контур без источника предлагает %q", tool.Name)
		}
	}
}

// A repository of the same shape as the ones this reads, and of nobody's: an index with a stage
// table and two items, served the way the forge serves them.
func docsFixture(t *testing.T) *docsboard.Source {
	t.Helper()

	files := map[string]string{
		"backlog.md": "# A lending system\n\n| Identifier | Stage |\n|---|---|\n| `stage-one` | First |\n",
		"backlog/B-01-holds.md": "---\nid: B-01\ntitle: \"Queue positions are recomputed on every read\"\n" +
			"status: done\npriority: P2\nsize: M\nstage: stage-one\n---\n",
		"backlog/B-02-notices.md": "---\nid: B-02\ntitle: \"A failed notice is recorded as sent\"\n" +
			"status: open\npriority: P1\nsize: S\nstage: stage-one\n---\n\n" +
			"A notice that was never delivered reads exactly like one that was.\n",
	}

	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	for name, body := range files {
		header := &tar.Header{Name: "example-docs-abc1234/" + name, Mode: 0o644, Size: int64(len(body))}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commits/") {
			w.Write([]byte("abc1234"))
			return
		}
		w.Write(buffer.Bytes())
	}))
	t.Cleanup(server.Close)

	return docsboard.New(docsboard.Config{
		Repo: "example/docs", Ref: "main", Root: "backlog", Index: "backlog.md",
		TTL: time.Hour, API: server.URL, Client: server.Client(),
	})
}
