package render

import "github.com/youndie/tacku/server/internal/domain"

// NavWidthDp is how wide the navigation is, and the board's empty placeholder was exactly this wide.
const NavWidthDp = 240

// Navigation is the same rail on every screen that is a destination in the graph.
//
// It used to be a method on the feed, so the feed had it and nothing else did. The board reserved
// the space — a 240dp column of `surface_block` with no children, named `board-nav-placeholder` —
// and a person who reached the board could not leave it: no catch-up, no my-tasks, no sign out, and
// nothing on the screen saying so. An empty rail looks like a design, which is why it survived.
//
// `current` is the destination this screen is, and it is the only argument that changes: the
// captions come from the graph, because that is where a destination is named. Spelled out here as
// well, they had already parted once — the graph said "Board" and the button beside it "Boards".
func Navigation(person domain.MemberID, current string) Component {
	return Column("nav", 4,
		[]Modifier{WidthDp(NavWidthDp), FillHeight(), Background(ColorSurfaceBlock), PaddingXY(20, 0)},
		Text("nav-brand", "tacku", TextTitle, PaddingXY(0, 20)),
		navItem("nav-catchup", LinkCatchUp, current),
		navItem("nav-boards", LinkBoard, current),
		navItem("nav-mine", LinkMyTasks, current),
		Spacer("nav-spacer"),
		Text("nav-person", string(person), TextMeta, PaddingXY(0, 20)),
		Button("nav-signout", "Sign out", Navigate(LinkSignOut), FillWidth(), PaddingXY(12, 20)),
	)
}

// navItem is one destination, and the highlight behind the current one spans the rail.
//
// It used to be as wide as the word: the selected block hugged its own text, so "Catch-up" carried a
// short grey tab instead of a highlighted row. Everything in this rail says `Fill` now, and that was
// available all along (Q-59).
func navItem(id, link, current string) Component {
	item := Button(id, RouteTitle(link), Navigate(link), FillWidth(), PaddingXY(12, 20))
	if link != current {
		return item
	}
	return Column(id+"-current", 0, []Modifier{FillWidth(), Background(ColorSurfaceSelected)}, item)
}
