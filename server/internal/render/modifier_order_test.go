package render_test

import (
	"encoding/json"
	"testing"

	"github.com/youndie/tacku/server/internal/render"
)

// The paint comes before the padding, on every node this package builds.
//
// Both orders are legal and both mean something, which is why this is a rule of the product rather
// than of the protocol: a card is a block of colour with its text inset, a screen is a surface that
// reaches the edge of the window. The other order leaves a margin — and a margin around a screen is
// the bare window, which on a desktop is white.
//
// It is checked rather than trusted because it was wrong on 52 nodes out of 53 and looked like six
// unrelated complaints: cards with no inner padding, menu items with no inner padding, a white frame
// around two forms. Nothing failed; every screen rendered.
func TestPaintComesBeforePadding(t *testing.T) {
	node := render.Column("c", 0, []render.Modifier{
		render.Padding(12),
		render.Background(render.ColorSurfaceField),
	})

	if order(t, node) != "background,padding" {
		t.Errorf("a column asked for padding then background emitted %q", order(t, node))
	}

	// A wrapper — padding and nothing to paint — is the way a gap is expressed where the parent has
	// no spacing of its own, so it must come through untouched.
	wrapper := render.Column("w", 0, []render.Modifier{render.Padding(6)})
	if order(t, wrapper) != "padding" {
		t.Errorf("a wrapper with only padding emitted %q", order(t, wrapper))
	}

	// Weight is neither, and its place in the chain does not change what is painted; what matters is
	// that it survives at all, because losing it changes the layout and nothing would say so.
	weighted := render.Column("g", 0, []render.Modifier{
		render.Weight(1),
		render.Padding(12),
		render.Background(render.ColorSurfaceBlock),
	})
	if got := order(t, weighted); got != "background,weight,padding" {
		t.Errorf("a weighted card emitted %q", got)
	}
}

func order(t *testing.T, node render.Component) string {
	t.Helper()

	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Modifiers []struct {
			Type string `json:"type"`
		} `json:"modifiers"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(decoded.Modifiers))
	for _, modifier := range decoded.Modifiers {
		names = append(names, modifier.Type)
	}
	return join(names)
}

func join(names []string) string {
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ","
		}
		out += name
	}
	return out
}
