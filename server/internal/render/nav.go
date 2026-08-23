package render

import (
	"strings"

	"github.com/youndie/tacku/server/internal/domain"
)

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
	// Никакого интервала: в макете пункты стоят вплотную (68, 109, 150 — ровно по 41), и высоту
	// строки задаёт её собственный отступ. Интервал в 4 сдвигал каждый следующий на 4 вниз.
	return Column("nav", 0,
		[]Modifier{WidthDp(NavWidthDp), FillHeight(), Background(ColorSurfaceBlock), PaddingXY(20, 0)},
		Text("nav-brand", "tacku", TextTitle, PaddingXY(12, 20)),
		navItem("nav-catchup", LinkCatchUp, current),
		navItem("nav-boards", LinkBoard, current),
		navItem("nav-mine", LinkMyTasks, current),
		Spacer("nav-spacer"),
		Text("nav-person", string(person), TextMeta, PaddingXY(0, 20)),
		Opens(
			Row("nav-signout", 0, []Modifier{FillWidth()},
				Text("nav-signout-label", "Sign out", TextNav, PaddingXY(12, 20))),
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
	modifiers := []Modifier{FillWidth()}
	if link == current {
		style = TextNavCurrent
		modifiers = append(modifiers, Background(ColorSurfaceSelected))
	}

	// The padding is on the label, not on the row, and that is the difference between an item you
	// can press and a word you can press. Modifiers apply in order and the click is added last, so
	// padding on the row insets the clickable area with everything else: the highlight covered the
	// rail and the target covered the text. The same mistake the button had, one level up.
	return Opens(
		Row(id, 0, modifiers, Text(id+"-label", RouteTitle(link), style, PaddingXY(12, 20))),
		Navigate(link),
	)
}

// Back is the way out of a screen you can only have arrived at.
//
// A form is opened from somewhere and answers by navigating away, so it never needed one — until
// somebody opened "New task" and decided not to create a task. There is no chrome around a screen
// in this product: if the way back is not in the tree, there is no way back.
//
// It goes **beside the action and before it**: the design draws "Back" to the left of "Continue",
// where a person is already looking when they decide which of the two they want. It was at the top
// for one build, which is where a browser would put it and this is not one. The padding is the
// mockup's — 12 by 18 against the action's 12 by 24 — so the two read as one pair.
//
// The caption comes from the graph, like every other destination's. Spelled here as well it had
// already parted once — the graph said "Board" and the button beside it "Boards" — and a way back
// whose word is invented is a second name for the same place.
// BackAction is the way out standing in a row of actions, where the design pads it like the button
// beside it: 12 by 18 against the action's 12 by 24, so the two read as one pair.
// BackAction is the way out standing beside the action that finishes the screen.
//
// A button rather than a padded line of text, and the reason is alignment: a button carries its own
// minimum height, so a label padded to look the same sat higher than the button beside it and no
// amount of padding fixed it honestly — the vocabulary has no way to say "centre these two". Two
// buttons in a row agree by construction. It reads as a link because the design system answers for
// a button with no variant with a transparent container and quiet text, which is the mechanism that
// exists precisely so appearance stays off the wire.
func BackAction(to string) Component {
	id := "back-" + strings.ToLower(strings.ReplaceAll(RouteTitle(to), " ", "-"))
	if RouteTitle(to) == "" {
		id = "back"
	}
	return Button(id, BackLabel(RouteTitle(to)), Navigate(to), PaddingXY(12, 18))
}

// BackTask is the way out of a screen that belongs to one task: back to that task, named by its
// identifier because that is what the person came from and what they will recognise.
//
// A button for the same reason [BackAction] is one — two buttons in a row line up and a padded line
// of text does not — and it was missed when that one changed, so the edit screen kept the crooked
// pair for another round. Its node is named after what it is rather than where it goes: the old
// identifier was built out of the deeplink, which put `app://task/tac-2` in the middle of a node
// name, and node names travel in update frames.
func BackTask(id domain.TaskID) Component {
	return Button("back-task", BackLabel(string(id)), Navigate(LinkTask+string(id)), PaddingXY(12, 18))
}

// BackLink is the way out standing above a title, where the design gives it no padding at all: a
// plain line at the content's left edge, 24 above the heading. Padding it there pushed the heading
// 33 points down the screen and indented the arrow away from everything under it.
func BackLink(to, destination string) Component { return backTo(to, destination, nil) }

// The destination is named by itself rather than by the route where there may be several of it.
//
// The task screen says "← Sprint 24" and not "← Board", because what a person left was that board
// and there may be several. The route's title is the right word everywhere the destination is the
// only one of its kind.
// The identifier is derived from the destination's title rather than from its deeplink: `app://` in
// the middle of a node name is a URL where a name should be, and node names travel in update frames
// and in logs.
func backTo(to, destination string, padding Modifier) Component {
	id := "back-" + strings.ToLower(strings.ReplaceAll(RouteTitle(to), " ", "-"))
	if RouteTitle(to) == "" {
		id = "back"
	}
	label := []Modifier{}
	if padding != nil {
		label = append(label, padding)
	}
	return Opens(
		Row(id, 0, nil,
			Text(id+"-label", BackLabel(destination), TextButtonQuiet, label...)),
		Navigate(to),
	)
}
