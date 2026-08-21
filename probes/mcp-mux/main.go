// Probe for research §1.5 and §1.7: one net/http mux serves both surfaces of tacku, and the
// agent-facing contract is generated from Go types the way the human-facing one is generated from
// Kotlin types.
//
// What a run proves:
//   - mcp.NewStreamableHTTPHandler is an ordinary http.Handler and mounts next to product routes,
//     so both surfaces live in one process behind one port and one middleware chain;
//   - the SDK derives inputSchema AND outputSchema from the Go argument and result types;
//   - a Go nil slice becomes "type":["null","array"], and generated schemas are
//     additionalProperties:false — the opposite default from the KOMPOT protocol (SPEC.md §3).
//
// Run: CGO_ENABLED=0 go run .
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listArgs struct {
	Since string `json:"since,omitempty" jsonschema:"opaque change cursor"`
}

type task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type listOut struct {
	Tasks  []task `json:"tasks"`
	Cursor string `json:"cursor"`
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "tacku", Version: "0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tasks",
		Description: "List tasks changed since a cursor",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listArgs) (*mcp.CallToolResult, listOut, error) {
		return nil, listOut{Tasks: []task{{ID: "TSK-1", Title: "write the spec"}}, Cursor: "c2"}, nil
	})

	mux := http.NewServeMux()
	// the human surface: kompot screens
	mux.HandleFunc("GET /screens/{screen}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"type":"column","id":%q,"children":[]}`, r.PathValue("screen"))
	})
	// the agent surface: MCP
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/screens/board")
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	fmt.Println("kompot screen:", body)

	client := mcp.NewClient(&mcp.Implementation{Name: "probe", Version: "0.0.1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if err != nil {
		panic(err)
	}
	defer session.Close()

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		panic(err)
	}
	for _, t := range tools.Tools {
		s, _ := json.Marshal(t.InputSchema)
		o, _ := json.Marshal(t.OutputSchema)
		fmt.Println("tool:", t.Name)
		fmt.Println("  inputSchema :", string(s))
		fmt.Println("  outputSchema:", string(o))
	}

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_tasks", Arguments: map[string]any{"since": "c1"}})
	if err != nil {
		panic(err)
	}
	sc, _ := json.Marshal(res.StructuredContent)
	fmt.Println("structuredContent:", string(sc))
}
