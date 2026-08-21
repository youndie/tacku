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

	// The prefix a task identifier follows. Client-known of necessity: the graph has no parameters.
	LinkTask = "app://task/"
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
var Graph = []Route{
	{Deeplink: LinkCatchUp, Endpoint: "/screens/catch-up", Kind: "screen", Title: "Catch-up"},
	{Deeplink: LinkBoard, Endpoint: "/screens/board", Kind: "screen", Title: "Board"},
	{Deeplink: LinkMyTasks, Endpoint: "/forms/my-tasks", Kind: "form", Title: "My tasks"},
	{Deeplink: LinkNewTask, Endpoint: "/forms/new-task", Kind: "form", Title: "New task"},
	{Deeplink: LinkNewBoard, Endpoint: "/forms/new-board", Kind: "form", Title: "New board"},
}

// ClientNative are the destinations a client resolves without the graph. Listed rather than left
// implicit, so that the test which demands every emitted deeplink resolve has something to check
// them against — an unlisted one is a dead button and not a special case.
var ClientNative = []string{LinkSignIn, LinkSignOut}

// ClientNativePrefixes are destinations that carry an identifier after them.
//
// Listed separately because the graph cannot express them at all: a route's endpoint is a literal
// path, so anything addressed by naming a thing is resolved by the client from a prefix it knows.
var ClientNativePrefixes = []string{LinkTask}
