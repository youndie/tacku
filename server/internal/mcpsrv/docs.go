package mcpsrv

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/youndie/tacku/server/internal/docsboard"
)

// The backlog of another repository, read and never written.
//
// Registered on the read-only server as well, because there is nothing here that could change
// anything: the items belong to a repository and change there, in a pull request. An agent asked to
// "add an item" has to say so to a person, and the absence of a tool is how it finds that out
// without spending a turn on a refusal.
func registerDocs(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "list_docs_items",
		Title: "List the items of the documented backlog",
		Description: "List the backlog kept as files in another repository — the same board a " +
			"person sees under \"Docs backlog\". Read-only: these items change in that repository, " +
			"through a pull request, and nothing here can move one. Filter by stage or status, or " +
			"ask for open only; use get_docs_item for the text of one.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in docsListIn) (*mcp.CallToolResult, docsListOut, error) {
		snapshot, err := deps.Docs.Load(ctx)
		if err != nil && snapshot.Empty() {
			return nil, docsListOut{}, err
		}

		out := docsListOut{
			Source: snapshot.Title,
			Items:  make([]docsBrief, 0, len(snapshot.Items)),
			ReadAt: snapshot.TakenAt.Format(time.RFC3339),
		}
		for _, item := range snapshot.Items {
			if in.Stage != "" && item.Stage != in.Stage {
				continue
			}
			if in.Status != "" && item.Status != in.Status {
				continue
			}
			if in.Open && item.Done() {
				continue
			}
			out.Items = append(out.Items, briefOfItem(item))
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:  "get_docs_item",
		Title: "Read one item of the documented backlog",
		Description: "The whole of one item: what it declares, what blocks it, and its text as " +
			"written — headings, lists and tables in the source's own markup. The path is where the " +
			"file sits in that repository, which is what to quote to somebody who has it checked out.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in docsGetIn) (*mcp.CallToolResult, docsFull, error) {
		snapshot, err := deps.Docs.Load(ctx)
		if err != nil && snapshot.Empty() {
			return nil, docsFull{}, err
		}

		item, found := snapshot.Item(in.ID)
		if !found {
			// Named rather than described: a model that invented an identifier needs to hear which
			// one did not exist, and one that quoted a stale one needs the same sentence.
			return nil, docsFull{}, fmt.Errorf("no item %q in this backlog", in.ID)
		}

		return nil, docsFull{
			docsBrief: briefOfItem(item),
			BlockedBy: append([]string{}, item.BlockedBy...),
			Epic:      item.Epic,
			Body:      item.Body,
			Path:      item.Path,
		}, nil
	})
}

func briefOfItem(item docsboard.Item) docsBrief {
	return docsBrief{
		ID:       item.ID,
		Title:    item.Title,
		Status:   item.Status,
		Priority: item.Priority,
		Size:     item.Size,
		Stage:    item.Stage,
	}
}
