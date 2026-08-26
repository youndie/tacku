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
		Description: "List the backlogs kept as files in other repositories — the same boards a " +
			"person sees in the navigation. Read-only: these items change in their own repository, " +
			"through a pull request, and nothing here can move one. Every item names the source it " +
			"came from, and an identifier is only unique inside one: B-01 is the first item of every " +
			"such backlog. Filter by source, stage or status, or ask for open only; use " +
			"get_docs_item with a source and an identifier for the text of one.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in docsListIn) (*mcp.CallToolResult, docsListOut, error) {
		out := docsListOut{Items: make([]docsBrief, 0), Sources: make([]docsSourceView, 0, len(deps.Docs))}

		for _, source := range deps.Docs {
			if in.Source != "" && source.Key() != in.Source {
				continue
			}

			snapshot, err := source.Load(ctx)
			if err != nil && snapshot.Empty() {
				// Named rather than dropped: a source that cannot be read is not a source with
				// nothing open, and an answer that quietly held back a whole repository would be
				// read as "nothing there".
				out.Sources = append(out.Sources, docsSourceView{
					Key: source.Key(), Title: source.Title(), Unread: err.Error(),
				})
				continue
			}

			out.Sources = append(out.Sources, docsSourceView{
				Key: source.Key(), Title: snapshot.Title, ReadAt: snapshot.TakenAt.Format(time.RFC3339),
			})
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
				out.Items = append(out.Items, briefOfItem(source.Key(), item))
			}
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
		source := deps.Docs.Find(in.Source)
		if source == nil {
			return nil, docsFull{}, fmt.Errorf("no backlog called %q here", in.Source)
		}

		snapshot, err := source.Load(ctx)
		if err != nil && snapshot.Empty() {
			return nil, docsFull{}, err
		}

		item, found := snapshot.Item(in.ID)
		if !found {
			// Named rather than described: a model that invented an identifier needs to hear which
			// one did not exist, and one that quoted a stale one needs the same sentence.
			return nil, docsFull{}, fmt.Errorf("no item %q in the backlog %q", in.ID, in.Source)
		}

		return nil, docsFull{
			docsBrief: briefOfItem(source.Key(), item),
			BlockedBy: append([]string{}, item.BlockedBy...),
			Epic:      item.Epic,
			Body:      item.Body,
			Path:      item.Path,
		}, nil
	})
}

func briefOfItem(source string, item docsboard.Item) docsBrief {
	return docsBrief{
		Source:   source,
		ID:       item.ID,
		Title:    item.Title,
		Status:   item.Status,
		Priority: item.Priority,
		Size:     item.Size,
		Stage:    item.Stage,
	}
}
