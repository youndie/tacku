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
		Opens(
			Row("nav-signout", 0, []Modifier{FillWidth(), PaddingXY(10, 16)},
				Text("nav-signout-label", "Sign out", TextNav)),
			Navigate(LinkSignOut),
		),
	)
}

// navItem is one destination: a line of text with a background behind the one you are standing on.
//
// A row of text rather than a button, and each half of that is a fix. A button centres its label and
// there is no alignment modifier to say otherwise, so a full-width one put every destination in the
// middle of the rail. A button also carries no style field, so its label could not be made heavier
// for the current item — and a highlight with no change of weight reads as decoration rather than as
// "you are here". Text takes a typography token, which carries both weight and colour.
//
// The padding is the item's own, and the highlight is that same node: one box, not a box inside a
// box, which is what made the selected item look inflated.
func navItem(id, link, current string) Component {
	style := TextNav
	modifiers := []Modifier{FillWidth(), PaddingXY(10, 16)}
	if link == current {
		style = TextNavCurrent
		modifiers = append(modifiers, Background(ColorSurfaceSelected))
	}

	return Opens(
		Row(id, 0, modifiers, Text(id+"-label", RouteTitle(link), style)),
		Navigate(link),
	)
}
