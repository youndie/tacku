package mcpsrv

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/idem"
)

// registerWrites adds the tools that change something.
//
// Every one of them runs through idem.Once with the caller's key, so a repeat replays the first
// outcome instead of producing a second. That protects the side effects as well as the operation:
// the journal entry is written inside the same transaction as the change, so replaying the outcome
// replays nothing at all.
func registerWrites(server *mcp.Server, deps Deps) {

	mcp.AddTool(server, &mcp.Tool{
		Name:  "create_task",
		Title: "Create a task",
		Description: "File a new task on a board. It is recorded as created by you on behalf of " +
			"the person you act for, and that pair is visible to the whole team.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createTaskIn) (*mcp.CallToolResult, taskOut, error) {
		by, err := deps.actor(ctx)
		if err != nil {
			return nil, taskOut{}, err
		}
		out, err := idem.Once(ctx, deps.Attempts, in.IdempotencyKey, in, func() (taskOut, error) {
			task, err := deps.Store.CreateTask(ctx, domain.Task{
				Board:    domain.BoardID(in.Board),
				Title:    in.Title,
				Body:     in.Body,
				Assignee: domain.MemberID(in.Assignee),
				Due:      in.Due,
			}, by)
			if err != nil {
				return taskOut{}, err
			}
			return taskOut{Task: briefOf(task)}, nil
		})
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:  "move_task",
		Title: "Move a task to another status",
		Description: "Change the status of one task. Moving it to the status it already has changes " +
			"nothing and is not recorded, so a repeat is safe but also invisible.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in moveTaskIn) (*mcp.CallToolResult, taskOut, error) {
		by, err := deps.actor(ctx)
		if err != nil {
			return nil, taskOut{}, err
		}
		out, err := idem.Once(ctx, deps.Attempts, in.IdempotencyKey, in, func() (taskOut, error) {
			// An agent has no screen, and that is a surface of its own rather than a blank: the
			// share of moves made from the task screen is counted over the two human surfaces, and
			// a tool call has to be visibly outside it rather than indistinguishable from a move
			// nobody recorded.
			task, err := deps.Store.MoveTask(ctx, domain.TaskID(in.Task), domain.Status(in.Status), by,
				domain.SurfaceAgent)
			if err != nil {
				return taskOut{}, err
			}
			return taskOut{Task: briefOf(task)}, nil
		})
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "assign_task",
		Title:       "Assign a task",
		Description: "Give a task to somebody, or take the assignee away by passing an empty one.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in assignTaskIn) (*mcp.CallToolResult, taskOut, error) {
		by, err := deps.actor(ctx)
		if err != nil {
			return nil, taskOut{}, err
		}
		out, err := idem.Once(ctx, deps.Attempts, in.IdempotencyKey, in, func() (taskOut, error) {
			task, err := deps.Store.AssignTask(ctx, domain.TaskID(in.Task), domain.MemberID(in.Assignee), by)
			if err != nil {
				return taskOut{}, err
			}
			return taskOut{Task: briefOf(task)}, nil
		})
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_due",
		Title:       "Set or clear the due date",
		Description: "Set the due date of a task as YYYY-MM-DD, or clear it by passing an empty date.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setDueIn) (*mcp.CallToolResult, taskOut, error) {
		by, err := deps.actor(ctx)
		if err != nil {
			return nil, taskOut{}, err
		}
		out, err := idem.Once(ctx, deps.Attempts, in.IdempotencyKey, in, func() (taskOut, error) {
			task, err := deps.Store.SetDue(ctx, domain.TaskID(in.Task), in.Due, by)
			if err != nil {
				return taskOut{}, err
			}
			return taskOut{Task: briefOf(task)}, nil
		})
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:  "comment_task",
		Title: "Comment on a task",
		Description: "Leave a comment. It is attributed to you acting for the person you serve, and " +
			"the team sees both.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in commentIn) (*mcp.CallToolResult, commentOut, error) {
		by, err := deps.actor(ctx)
		if err != nil {
			return nil, commentOut{}, err
		}
		out, err := idem.Once(ctx, deps.Attempts, in.IdempotencyKey, in, func() (commentOut, error) {
			comment, err := deps.Store.Comment(ctx, domain.TaskID(in.Task), in.Text, by)
			if err != nil {
				return commentOut{}, err
			}
			return commentOut{Comment: commentView{
				Text: comment.Text, By: actorOf(comment.By), At: comment.CreatedAt.Format(stamp),
			}}, nil
		})
		return nil, out, err
	})
}

type commentOut struct {
	Comment commentView `json:"comment"`
}
