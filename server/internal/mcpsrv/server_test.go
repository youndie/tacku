package mcpsrv_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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

func start(t *testing.T) harness {
	t.Helper()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "tacku.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	server, err := mcpsrv.New(mcpsrv.Deps{
		Store: store, Attempts: store,
		Agent: robot, Version: "0.1.0", OnBehalfOf: anna,
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
