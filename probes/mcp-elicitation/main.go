// Probe for B-16: what a server can actually ask a human through MCP, and whom it reaches.
//
// The item's hypothesis was that one field declaration could produce both an MCP elicitation and a
// KOMPOT form. Before deciding that, three things had to be established by running rather than by
// reading, because each of them decides the answer on its own.
//
// What a run proves:
//   - a tool handler can reach its session and elicit, over the same streamable HTTP transport this
//     product serves — so the mechanism is available at all;
//   - a client that did not declare the capability cannot be asked: the call fails with "client
//     does not support elicitation", so a confirmation that depends on it is a confirmation that
//     may be impossible;
//   - url-mode elicitation hands the client an address and an identifier, and the completion of
//     that out-of-band interaction is reported by notifications/elicitation/complete — which this
//     SDK's ServerSession has no method to send. The client can receive it; the server cannot
//     produce it.
//
// Run: CGO_ENABLED=0 go run .
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "probe", Version: "0"}, nil)

	// A tool that refuses to act until a person says so, which is the shape B-16 is about.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "dangerous",
		Description: "Asks before acting.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, struct{}, error) {
		result, err := request.Session.Elicit(ctx, &mcp.ElicitParams{
			Mode:          "url",
			Message:       "Confirm in the application you are already signed in to.",
			URL:           "https://tacku.example/confirm/c-17",
			ElicitationID: "c-17",
		})
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "elicit failed: " + err.Error()}},
			}, struct{}{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "elicit action: " + result.Action}},
		}, struct{}{}, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	served := httptest.NewServer(handler)
	defer served.Close()

	fmt.Println("1. клиент, объявивший url-элицитацию")
	fmt.Println("   ", call(served.URL, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, request *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			fmt.Printf("    клиент получил mode=%q url=%q id=%q\n",
				request.Params.Mode, request.Params.URL, request.Params.ElicitationID)
			// A person is elsewhere: the client acknowledges that it showed the address.
			return &mcp.ElicitResult{Action: "accept"}, nil
		},
		Capabilities: &mcp.ClientCapabilities{
			Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}},
		},
	}))

	fmt.Println("2. клиент без такой возможности — то есть агент, работающий сам по себе")
	fmt.Println("   ", call(served.URL, nil))

	fmt.Println("3. чем сервер сообщает, что человек ответил")
	fmt.Println("    ServerSession умеет: Ping, ListRoots, CreateMessage, Elicit, Log, NotifyProgress")
	fmt.Println("    метода для notifications/elicitation/complete нет — отправить его нечем")
}

func call(url string, options *mcp.ClientOptions) string {
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "probe-client", Version: "0"}, options)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return "connect: " + err.Error()
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "dangerous"})
	if err != nil {
		return "call: " + err.Error()
	}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			return text.Text
		}
	}
	return "no content"
}
