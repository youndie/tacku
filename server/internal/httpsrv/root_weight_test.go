package httpsrv_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// No screen puts a weight among the children of a root that is a column.
//
// The client draws a screen so that it scrolls when it outgrows the window, and that mechanism lays
// the root's children out as separate items. A `weight` among them therefore has nothing to divide
// and collapses to nothing — while §5.2 says a weighted node is interpreted by its parent and takes
// its share, which is true everywhere else in the same tree.
//
// It cost one screen. Centring is two weighted spacers above and below, and the sign-in form was the
// only screen with a column at the root, so it was the only one that ended up against the top edge
// with its heading clipped. Every other screen has a row at the root and never showed it. The remedy
// is a row at the root: the whole column then sits inside one item, where weight means what it means
// everywhere else.
//
// The rule is narrow, and its first version was not: it flagged every weighted child of every root
// and lit up four screens that were correct. A row at the root is one item, and its children lay
// out normally — the weights inside it are exactly the ones that do work. A check that fires on
// what is right teaches people to ignore it.
func TestNoScreenWeighsTheRootsOwnChildren(t *testing.T) {
	r := newResource(t)
	r.fill(t, 2)
	token := r.reader(t)

	screens := screenPaths()

	checked := 0
	for _, path := range screens {
		response, body := r.get(t, path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s answered %d", path, response.StatusCode)
		}

		var envelope map[string]any
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Fatal(err)
		}
		// A form answers {schema, screen}; a screen answers the tree itself.
		root := envelope
		if inner, ok := envelope["screen"].(map[string]any); ok {
			root = inner
		}

		if root["type"] != "column" {
			// A row at the root is one item and lays its children out itself, which is the remedy
			// rather than the disease.
			continue
		}

		children, ok := root["children"].([]any)
		if !ok {
			t.Errorf("%s: the root holds no children, so this check looked at nothing", path)
			continue
		}
		checked++

		for _, child := range children {
			value, ok := child.(map[string]any)
			if !ok {
				continue
			}
			if weighted(value) {
				id, _ := value["id"].(string)
				t.Errorf("%s: %q carries a weight and is a child of the root, where the client lays children out as separate items: the weight divides nothing and the node collapses",
					path, id)
			}
		}
	}

	// Counted, because the whole set could drift to rows at the root and this check would then pass
	// while looking at nothing — and it is exactly the screens with a column at the root that it
	// exists for.
	if checked == 0 {
		t.Fatal("not one screen has a column at its root, so this check looked at nothing")
	}
	t.Logf("%d screens with a column at the root", checked)
}

func weighted(node map[string]any) bool {
	modifiers, ok := node["modifiers"].([]any)
	if !ok {
		return false
	}
	for _, modifier := range modifiers {
		value, ok := modifier.(map[string]any)
		if ok && value["type"] == "weight" {
			return true
		}
	}
	return false
}
