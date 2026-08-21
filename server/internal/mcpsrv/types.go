package mcpsrv

import (
	"github.com/youndie/tacku/server/internal/domain"
)

// The shapes below are the agent-facing contract. The SDK derives inputSchema and outputSchema from
// them, so a field renamed here is a protocol change for every model talking to this server —
// exactly as a renamed wire type would be on the human side.
//
// Slices are always returned non-nil. A nil slice marshals to null, and the generated schema then
// says ["null","array"]; a client validating strictly has to handle both for no reason.

// taskBrief is the default projection: what a list is for.
//
// Deliberately narrow. A list of forty tasks carrying full descriptions costs the model more than
// it costs the team, and the body is one call away by identifier.
type taskBrief struct {
	ID       string `json:"id" jsonschema:"the identifier people quote, for example TAC-124"`
	Title    string `json:"title"`
	Status   string `json:"status" jsonschema:"one of todo, in_progress, in_review, done, blocked"`
	Assignee string `json:"assignee,omitempty"`
	Board    string `json:"board"`
}

type taskFull struct {
	taskBrief
	Body     string        `json:"body,omitempty"`
	Due      string        `json:"due,omitempty" jsonschema:"YYYY-MM-DD, empty when no date is set"`
	Comments []commentView `json:"comments"`
}

type commentView struct {
	Text string `json:"text"`
	By   actor  `json:"by"`
	At   string `json:"at" jsonschema:"RFC 3339"`
}

// actor is the pair the product is built on. It appears in every read that shows history, because
// a change whose origin is not visible is a change the team has to verify by hand.
type actor struct {
	Kind       string `json:"kind" jsonschema:"human or agent"`
	Member     string `json:"member"`
	Version    string `json:"version,omitempty" jsonschema:"the agent build that acted; empty for a person"`
	OnBehalfOf string `json:"on_behalf_of"`
}

type changeView struct {
	Task string `json:"task"`
	Kind string `json:"kind"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	By   actor  `json:"by"`
	At   string `json:"at" jsonschema:"RFC 3339"`
}

func briefOf(t domain.Task) taskBrief {
	return taskBrief{
		ID:       string(t.ID),
		Title:    t.Title,
		Status:   string(t.Status),
		Assignee: string(t.Assignee),
		Board:    string(t.Board),
	}
}

func actorOf(p domain.Provenance) actor {
	return actor{
		Kind:       string(p.Executor.Kind),
		Member:     string(p.Executor.Member),
		Version:    p.Executor.Version,
		OnBehalfOf: string(p.OnBehalfOf),
	}
}
