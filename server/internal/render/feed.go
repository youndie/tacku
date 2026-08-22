package render

import (
	"fmt"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

// Feed builds the catch-up screen: what changed since the reader last looked, and who did it.
//
// The one screen of this product that carries no input, and therefore the one that is a `screen`
// rather than a `form` — which is also why it is the only one conditional delivery applies to.
type Feed struct {
	Person   domain.MemberID
	SeenURL  string
	Changes  []domain.Change
	NextPage string

	// What the headline counts, and it is not what the list shows.
	//
	// Both used to be taken from what was at hand: the length of the first page, and the number of
	// boards in the workspace. Read out loud that sentence said "20 changes across 4 boards" to
	// somebody with two hundred waiting on one — every word of it wrong, and none of it visibly so,
	// because a plausible number beside a list of twenty rows looks like a count of that list.
	Total  int
	Boards int

	// Away is how long this reader had been gone when this visit began; zero for somebody who has
	// never been here, and the headline then says nothing about a previous visit.
	Away time.Duration
}

// Screen renders the tree.
//
// Every identifier here is derived from something stable — a journal sequence, a fixed name — and
// never from a counter over the walk. Conditional delivery compares bodies, so an identifier that
// changed per request would make the ETag change per request, and a 304 would never happen.
func (f Feed) Screen() Component {
	return Row("screen-catchup", 0,
		[]Modifier{Background(ColorSurface)},
		f.navigation(),
		Rule("nav-rule", RuleDp, ColorDivider, false),
		f.body(),
	)
}

func (f Feed) navigation() Component {
	return Column("nav", 4,
		[]Modifier{WidthDp(240), Background(ColorSurfaceBlock), PaddingXY(20, 0)},
		Text("nav-brand", "tacku", TextTitle, PaddingXY(0, 20)),
		// The captions come from the graph, which is where a destination is named. Spelled here as
		// well, they had already parted: the graph said "Board" and the button beside it "Boards".
		Column("nav-current", 0, []Modifier{Background(ColorSurfaceSelected)},
			Button("nav-catchup", RouteTitle(LinkCatchUp), Navigate(LinkCatchUp), PaddingXY(12, 20)),
		),
		Button("nav-boards", RouteTitle(LinkBoard), Navigate(LinkBoard), PaddingXY(12, 20)),
		Button("nav-mine", RouteTitle(LinkMyTasks), Navigate(LinkMyTasks), PaddingXY(12, 20)),
		Spacer("nav-spacer"),
		Text("nav-person", string(f.Person), TextMeta, PaddingXY(0, 20)),
		Button("nav-signout", "Sign out", Navigate(LinkSignOut), PaddingXY(12, 20)),
	)
}

func (f Feed) body() Component {
	return Column("feed", 24,
		[]Modifier{Weight(1), Padding(32)},
		f.header(),
		PaginatedList("feed-list", f.rows(), f.NextPage, emptyFeed(), Weight(1)),
	)
}

func (f Feed) header() Component {
	return Row("feed-header", 0, nil,
		Column("feed-heading", 6, nil,
			Text("feed-title", "Since your last visit", TextDisplay),
			Text("feed-count", FeedSummary(f.Total, f.Boards, f.Away), TextBodyMuted),
		),
		Spacer("feed-header-spacer"),
		// A perform and not a navigate: marking everything seen changes state, and it used to be a
		// navigate to the same screen with a query on the end — a state change wearing the clothes
		// of navigation, which is also a deeplink the graph could never carry.
		Button("feed-seen", "Mark all as seen", Perform(f.SeenURL, nil), PaddingXY(12, 20)),
	)
}

func (f Feed) rows() []Component {
	items := make([]Component, 0, len(f.Changes))
	for _, change := range f.Changes {
		items = append(items, ChangeRow(change))
	}
	return items
}

// ChangeRow is one entry of the journal, and it carries provenance twice on purpose: a stripe for
// the eye scanning a column, and a line naming both actors for the reader who stops.
func ChangeRow(change domain.Change) Component {
	stripe := ColorDivider
	authorStyle := TextMeta
	if change.By.ByAgent() {
		stripe = ColorAgent
		authorStyle = TextMetaAgent
	}

	id := fmt.Sprintf("change-%d", change.Seq)
	body := Row(id, 0, nil,
		Column(id+"-stripe", 0, []Modifier{WidthDp(StripeDp), Background(stripe)}),
		Column(id+"-body", 6,
			[]Modifier{Weight(1), Padding(16), Background(ColorSurfaceBlock)},
			Text(id+"-what", Sentence(change), TextBody),
			Text(id+"-who", Author(change), authorStyle),
		),
	)
	// The whole row opens the task, which is what the design drew and what the vocabulary could
	// not say until kompot 0.15. It was a button per entry for exactly one release (Q-22).
	return Opens(body, Navigate(LinkTask+string(change.Task)))
}

func emptyFeed() Component {
	return Column("feed-empty", 8,
		[]Modifier{Padding(32), Background(ColorSurfaceBlock)},
		Text("feed-empty-title", "Nothing changed while you were away", TextTitle),
		Text("feed-empty-body",
			"You are up to date. Anything your agent does on your behalf will show up here first.",
			TextBodyMuted),
		Row("feed-empty-actions", 0, nil,
			Button("feed-empty-go", "Go to your boards", Navigate(LinkBoard), PaddingXY(12, 20)),
			Spacer("feed-empty-spacer"),
		),
	)
}
