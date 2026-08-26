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

// docsBrief is one item of a backlog kept in another repository, as a list shows it.
//
// The fields are the source's own words and are passed through rather than mapped onto this
// product's vocabulary: a status outside the method's list, a size spelled `S/M` and a priority
// that is a word all occur in a live repository, and a value quietly rewritten into the nearest
// local one would be a lie a model cannot check.
type docsBrief struct {
	// Source is which backlog this came from, and it is not decoration: an identifier is unique
	// inside one repository and nowhere else — B-01 is the first item of every backlog written by
	// this method.
	Source   string `json:"source"`
	ID       string `json:"id" jsonschema:"the identifier the repository quotes, for example B-171"`
	Title    string `json:"title"`
	Status   string `json:"status" jsonschema:"as the source writes it: usually open, wip, done, question or dropped"`
	Priority string `json:"priority,omitempty"`
	Size     string `json:"size,omitempty"`
	Stage    string `json:"stage,omitempty" jsonschema:"the stage this item belongs to, which is what the board uses as a column"`
}

type docsFull struct {
	docsBrief
	BlockedBy []string `json:"blockedBy"`
	Epic      string   `json:"epic,omitempty"`
	// Body is the item as written, in the source's own markup.
	Body string `json:"body,omitempty"`
	// Path is where the file sits in that repository, so that a person told about it can open it.
	Path string `json:"path"`
}

type docsListOut struct {
	// Sources are the backlogs this answer covers, each with what it calls itself and when it was
	// last read — these are cached copies of somebody else's repositories, and "how old" is a
	// question the caller is entitled to. A source that could not be read says so here rather than
	// being left out, because an answer missing a whole repository reads as "nothing there".
	Sources []docsSourceView `json:"sources"`
	Items   []docsBrief      `json:"items"`
}

type docsSourceView struct {
	Key    string `json:"key" jsonschema:"what to pass as source to get_docs_item"`
	Title  string `json:"title"`
	ReadAt string `json:"readAt,omitempty" jsonschema:"RFC 3339"`
	Unread string `json:"unread,omitempty" jsonschema:"why this backlog could not be read, when it could not"`
}

type docsListIn struct {
	Source string `json:"source,omitempty" jsonschema:"only this backlog, by its key; all of them when absent"`
	Stage  string `json:"stage,omitempty" jsonschema:"only items of this stage"`
	Status string `json:"status,omitempty" jsonschema:"only items with this status, as the source spells it"`
	Open   bool   `json:"open,omitempty" jsonschema:"only what is not done, which is usually what is being asked"`
}

type docsGetIn struct {
	Source string `json:"source" jsonschema:"which backlog, by the key list_docs_items reports"`
	ID     string `json:"id" jsonschema:"the identifier inside that backlog, for example B-171"`
}
