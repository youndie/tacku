package render

import "strings"

// Where a screen lives, in one place.
//
// Enumerating what this server actually emitted turned up two spellings of one destination —
// `app://board` from a button and `app://boards` from the navigation panel — and one of them
// resolved to nothing. A deeplink the graph does not carry is one the client is required to ignore
// (§12.2), so the failure is a button that does nothing, in silence, for as long as nobody presses
// it while watching.
//
// The constants exist so that a destination is spelled once. The test that walks every rendered tree
// and demands each deeplink resolve exists because constants are a convention and a test is not.
const (
	LinkCatchUp  = "app://catch-up"
	LinkBoard    = "app://board"
	LinkMyTasks  = "app://my-tasks"
	LinkNewTask  = "app://new-task"
	LinkNewBoard = "app://new-board"

	// The selection mode, which is a screen because the vocabulary has no mode. See render.BulkMove.
	LinkBulkMove = "app://bulk-move"

	// The read-only view over a backlog kept in a repository, with the source's key after it.
	// Carried once per configured source — see [WithDocsBoards].
	LinkDocsBoard = "app://docs-board/"

	// One item of a backlog: the source's key, then the identifier. Resolved by the client from a
	// prefix, like a task.
	//
	// The key is in the address because identifiers are a repository's own and collide across them:
	// B-01 is the first item of every backlog written by this method.
	LinkDocsItem = "app://docs-item/"

	// Known to the client rather than carried by the graph.
	//
	// Signing in cannot be in the graph by construction: fetching the graph needs a session, and
	// somebody who needs to sign in has none. Signing out is not a screen at all — it is an act,
	// and the protocol has no action for it, so a navigate is the nearest thing.
	LinkSignIn  = "app://sign-in"
	LinkSignOut = "app://sign-out"

	// The prefix a task identifier follows. Client-known of necessity: the graph has no parameters.
	LinkTask = "app://task/"

	// The same for the form that edits one. A second prefix rather than a query on the first,
	// because the graph carries neither and the client has to recognise both by shape anyway.
	LinkEditTask = "app://edit-task/"
)

// Route is one entry of the navigation graph.
//
// Kind says what stands behind the endpoint, in the vocabulary of x-kompot-endpoint-kind, and it
// must equal the kind the HTTP description declares for that path. Absent means "screen", so a
// route written before the field keeps its meaning.
type Route struct {
	Deeplink string `json:"deeplink"`
	Endpoint string `json:"endpoint"`
	Kind     string `json:"kind,omitempty"`
	Title    string `json:"title,omitempty"`

	// Rail is the node this destination gets in the navigation rail, and empty means it has none.
	//
	// On the route rather than in the rail, because the rail used to list three destinations by
	// hand out of the six the graph carried, and a deployment that carries a seventh has no way to
	// say so without editing a renderer. It travels on the wire with the rest of the route, which
	// is harmless — a client that does not know the field ignores it — and it keeps the two facts
	// about a destination, that it exists and that a person can get to it, in one place.
	Rail string `json:"rail,omitempty"`
}

// Graph is what the client fetches to learn which screens it can reach without a release of its own.
//
// It carries forms again. It could not until kompot 0.13: a route promised a tree, a form answers an
// envelope, and `ScreenRoute` had no way to say which — so a screen taking input needed a client
// release, which for a tracker is most of them. The answer upstream added `kind`, an open string
// rather than an enum, precisely so that a client can skip a route it does not understand instead of
// failing to parse the graph around it.
//
// The rollout caveat that comes with it does not bite here: a client shipped before the field would
// decode a form as a tree, and this product has no shipped client at all.
//
// A per-task screen is still absent, and for a different reason: `endpoint` is a literal path and the
// graph has no parameters, so a screen addressed by an identifier is always one the client builds.
var base = []Route{
	{Deeplink: LinkCatchUp, Endpoint: "/screens/catch-up", Kind: "screen", Title: "Catch-up", Rail: "nav-catchup"},
	{Deeplink: LinkBoard, Endpoint: "/screens/board", Kind: "screen", Title: "Board", Rail: "nav-boards"},
	{Deeplink: LinkMyTasks, Endpoint: "/forms/my-tasks", Kind: "form", Title: "My tasks", Rail: "nav-mine"},
	{Deeplink: LinkBulkMove, Endpoint: "/forms/bulk-move", Kind: "form", Title: "Move several"},
	{Deeplink: LinkNewTask, Endpoint: "/forms/new-task", Kind: "form", Title: "New task"},
	{Deeplink: LinkNewBoard, Endpoint: "/forms/new-board", Kind: "form", Title: "New board"},
}

// Graph is what this deployment carries. Assembled before the server starts serving and not touched
// afterwards.
var Graph = base

// DocsRoute is the read-only view over a backlog kept in a repository.
//
// Defined always and carried only where a source is configured. The two halves of carrying it — the
// route and the endpoint behind it — are decided in one place (httpsrv.New) because a route
// promising an endpoint nobody registered is a button that does nothing, in silence, for as long as
// nobody presses it while watching.
// DocsBoardPath is where a board is served, with the source's key after it, and DocsItemPath what a
// key and an identifier go after. Constants because the route and the handler must agree on them,
// and they are registered in two packages.
const (
	DocsBoardPath = "/screens/docs-board/"
	DocsItemPath  = "/screens/docs-item/"
)

// DocsBoardRoute is the route of one source's board.
//
// One per source rather than one screen taking a parameter, because the graph carries literal paths
// and has none. That is not a workaround: which backlogs a deployment shows is exactly the kind of
// thing a graph is for, and each gets its own entry in the rail under the name its configuration
// gives it.
func DocsBoardRoute(key, title string) Route {
	return Route{
		Deeplink: LinkDocsBoard + key,
		Endpoint: DocsBoardPath + key,
		Kind:     "screen",
		Title:    title,
		Rail:     "nav-docs-" + key,
	}
}

// WithDocsBoards assembles the graph of a deployment showing these backlogs, in this order.
//
// It assigns rather than appends, and that is the whole reason it is written out: a process that
// builds a server twice — which every test binary does — would otherwise carry a route twice, or
// carry one in a server configured without the source behind it. Assigning makes the graph a
// function of the argument instead of a function of the history.
func WithDocsBoards(sources []Route) {
	Graph = append(append([]Route(nil), base...), sources...)
	ClientNativePrefixes = basePrefixes
	if len(sources) > 0 {
		ClientNativePrefixes = append(append([]string(nil), basePrefixes...), LinkDocsItem)
	}
}

// ClientNative are the destinations a client resolves without the graph. Listed rather than left
// implicit, so that the test which demands every emitted deeplink resolve has something to check
// them against — an unlisted one is a dead button and not a special case.
var ClientNative = []string{LinkSignIn, LinkSignOut}

// ClientNativePrefixes are destinations that carry an identifier after them.
//
// Listed separately because the graph cannot express them at all: a route's endpoint is a literal
// path, so anything addressed by naming a thing is resolved by the client from a prefix it knows.
//
// Assembled with the graph and for the same reason: the item screen exists only where a source
// does, and a prefix listed by a deployment that cannot serve it is an address that answers 404
// after the client has already navigated.
var ClientNativePrefixes = basePrefixes

var basePrefixes = []string{LinkTask, LinkEditTask}

// ClientPaths are the addresses a page can be standing at, as a browser writes them.
//
// The same names as the deeplinks with the scheme taken off — a translation rather than a second
// routing table, because two tables drift and a person then has a link that works in one of them.
// The server needs them for one thing: answering with the page when somebody reloads or follows a
// link, instead of the 404 that a path with no file behind it would otherwise get.
func ClientPaths() ([]string, []string) {
	exact := make([]string, 0, len(Graph)+len(ClientNative))
	for _, route := range Graph {
		exact = append(exact, pathOfDeeplink(route.Deeplink))
	}
	for _, deeplink := range ClientNative {
		exact = append(exact, pathOfDeeplink(deeplink))
	}

	prefixes := make([]string, 0, len(ClientNativePrefixes))
	for _, deeplink := range ClientNativePrefixes {
		prefixes = append(prefixes, pathOfDeeplink(deeplink))
	}
	return exact, prefixes
}

func pathOfDeeplink(deeplink string) string { return "/" + strings.TrimPrefix(deeplink, "app://") }
