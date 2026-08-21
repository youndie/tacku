package render

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

	// Known to the client rather than carried by the graph.
	//
	// Signing in cannot be in the graph by construction: fetching the graph needs a session, and
	// somebody who needs to sign in has none. Signing out is not a screen at all — it is an act,
	// and the protocol has no action for it, so a navigate is the nearest thing.
	LinkSignIn  = "app://sign-in"
	LinkSignOut = "app://sign-out"
)

// Route is one entry of the navigation graph.
type Route struct {
	Deeplink string `json:"deeplink"`
	Endpoint string `json:"endpoint"`
	Title    string `json:"title,omitempty"`
}

// Graph is what the client fetches to learn which screens it can reach without a release of its own.
//
// Only endpoints of kind `screen`, and this is narrower than it first looks. A route promises that
// fetching the endpoint yields a tree (§12.1), and `ScreenRoute` carries `deeplink`, `endpoint` and
// `title` — no kind. So a client has no way to know whether to parse what comes back as a component
// or as a form envelope, and the conformance kit is right to assume the former: three form routes
// put here answered "no discriminator property type", because a form response is an envelope with a
// tree inside rather than a tree.
//
// The consequence is worth stating rather than working around: **a form screen cannot be added
// without a client release.** For a product whose screens are mostly forms — a tracker being one —
// that is a real limit, and it is reported upstream rather than absorbed quietly.
//
// A per-task screen is absent for a second reason: `endpoint` is a literal path and the graph has no
// parameters, so a screen addressed by an identifier is always one the client builds itself.
var Graph = []Route{
	{Deeplink: LinkCatchUp, Endpoint: "/screens/catch-up", Title: "Catch-up"},
	{Deeplink: LinkBoard, Endpoint: "/screens/board", Title: "Board"},
}

// ClientNative are the destinations a client resolves without the graph. Listed rather than left
// implicit, so that the test which demands every emitted deeplink resolve has something to check
// them against — an unlisted one is a dead button and not a special case.
var ClientNative = []string{
	LinkSignIn, LinkSignOut,

	// Form screens, which the graph cannot carry: see Graph above. Every one of these needs the
	// client to know it, which is the cost the protocol charges for a screen that takes input.
	LinkMyTasks, LinkNewTask, LinkNewBoard,
}
