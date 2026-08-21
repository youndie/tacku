// Package mcpsrv exposes the tracker to AI agents over the Model Context Protocol.
//
// Two properties of MCP 2026-07-28 shape everything here. There is no protocol-level session, so
// nothing is remembered between calls and every operation is addressed by an explicit identifier.
// And tool descriptions are the interface: they are what the model reads when deciding which call
// to make, so they are edited like code and not like comments.
//
// Credentials come from the environment, which is what the specification asks of a stdio server —
// the HTTP transport would make this an OAuth 2.1 resource server, and that is B-17.
package mcpsrv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/youndie/tacku/server/internal/domain"
)

// Deps is what the tools need. Two interfaces rather than one concrete store, so the tests can see
// what is actually used.
type Deps struct {
	Store    domain.Store
	Attempts domain.Attempts

	// Identity of the agent this process serves, and the person it acts for. Taken from the
	// environment at start-up: over stdio the agent runs beside its principal.
	Agent      domain.MemberID
	Version    string
	OnBehalfOf domain.MemberID
}

func (d Deps) provenance() domain.Provenance {
	return domain.Agent(d.Agent, d.Version, d.OnBehalfOf)
}

func (d Deps) validate() error {
	if d.Store == nil || d.Attempts == nil {
		return fmt.Errorf("mcpsrv: no store")
	}
	return d.provenance().Validate()
}

// New builds the server with every tool registered.
func New(deps Deps) (*mcp.Server, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "tacku", Version: deps.Version}, nil)
	registerReads(server, deps)
	registerWrites(server, deps)
	return server, nil
}

// RunStdio serves on stdin and stdout until the client disconnects.
func RunStdio(ctx context.Context, deps Deps) error {
	server, err := New(deps)
	if err != nil {
		return err
	}
	session, err := server.Connect(ctx, &mcp.StdioTransport{}, nil)
	if err != nil {
		return err
	}

	// A host closing the pipe is how an MCP session ends, not how it fails. Returning the EOF makes
	// the process exit non-zero, and under a supervisor an ordinary disconnect then looks like a
	// crash — with a restart loop to match.
	if err := session.Wait(); err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "EOF") {
		return err
	}
	return nil
}

func registerReads(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "list_boards",
		Title: "List boards",
		Description: "List the boards of this workspace. Cheap, and the board identifier is what " +
			"every other call needs.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, boardsOut, error) {
		boards, err := deps.Store.Boards(ctx)
		if err != nil {
			return nil, boardsOut{}, err
		}
		out := boardsOut{Boards: make([]boardView, 0, len(boards))}
		for _, b := range boards {
			out.Boards = append(out.Boards, boardView{ID: string(b.ID), Title: b.Title})
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:  "list_tasks",
		Title: "List tasks on a board",
		Description: "List the tasks of one board, briefly: identifier, title, status, assignee. " +
			"Use get_task for the description and the comments of one task. If you are looking for " +
			"what happened since you last looked, changes_since is cheaper and exact — this call " +
			"returns everything and leaves the comparing to you.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listTasksIn) (*mcp.CallToolResult, tasksOut, error) {
		tasks, err := deps.Store.Tasks(ctx, domain.BoardID(in.Board))
		if err != nil {
			return nil, tasksOut{}, err
		}
		out := tasksOut{Tasks: make([]taskBrief, 0, len(tasks))}
		for _, t := range tasks {
			if in.Status != "" && string(t.Status) != in.Status {
				continue
			}
			out.Tasks = append(out.Tasks, briefOf(t))
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_task",
		Title:       "Read one task in full",
		Description: "The description and the full comment history of one task, with the author of each entry.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in taskRef) (*mcp.CallToolResult, taskFull, error) {
		task, err := deps.Store.Task(ctx, domain.TaskID(in.Task))
		if err != nil {
			return nil, taskFull{}, err
		}
		comments, err := deps.Store.Comments(ctx, task.ID)
		if err != nil {
			return nil, taskFull{}, err
		}
		out := taskFull{taskBrief: briefOf(task), Body: task.Body, Due: task.Due, Comments: make([]commentView, 0, len(comments))}
		for _, c := range comments {
			out.Comments = append(out.Comments, commentView{Text: c.Text, By: actorOf(c.By), At: c.CreatedAt.Format(stamp)})
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:  "changes_since",
		Title: "What changed since a cursor",
		Description: "Everything that happened after the cursor you were given last time, oldest " +
			"first, with who did it and on whose behalf. Pass an empty cursor the first time. This " +
			"is the cheap way to stay current: re-listing boards and comparing them yourself costs " +
			"far more and can still miss a change that was undone. Keep the returned cursor for " +
			"your next call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in changesIn) (*mcp.CallToolResult, changesOut, error) {
		cursor := domain.Cursor(in.Cursor)
		if in.Cursor == "" {
			cursor = domain.Start
		}
		changes, next, err := deps.Store.Changes(ctx, cursor, in.Limit)
		if err != nil {
			return nil, changesOut{}, err
		}
		out := changesOut{Cursor: string(next), Changes: make([]changeView, 0, len(changes))}
		for _, c := range changes {
			out.Changes = append(out.Changes, changeView{
				Task: string(c.Task), Kind: string(c.Kind), From: c.From, To: c.To,
				By: actorOf(c.By), At: c.CreatedAt.Format(stamp),
			})
		}
		return nil, out, nil
	})
}

const stamp = "2006-01-02T15:04:05Z07:00"
