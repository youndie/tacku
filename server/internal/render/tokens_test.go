package render_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/youndie/tacku/server/internal/render"
)

// The two halves of the token set, compared.
//
// The client declares the names (§6) and this server has to use exactly those. They cannot share
// code — two languages — so they share a file, and this is the check that makes the file mean
// something. Without it a name only one side knows costs a default and a warning: the screen
// renders, in the wrong colours, and the log says so where nobody is looking.
func TestTheServerUsesTheNamesTheClientDeclares(t *testing.T) {
	var published struct {
		Colors     []string `json:"colors"`
		Typography []string `json:"typography"`
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "design", "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &published); err != nil {
		t.Fatal(err)
	}

	compare(t, "colour", published.Colors, render.ColorTokens())
	compare(t, "typography", published.Typography, render.TypographyTokens())
}

func compare(t *testing.T, kind string, declared, used []string) {
	t.Helper()

	sort.Strings(declared)
	sort.Strings(used)
	if reflect.DeepEqual(declared, used) {
		return
	}

	inClient := map[string]bool{}
	for _, name := range declared {
		inClient[name] = true
	}
	for _, name := range used {
		if !inClient[name] {
			t.Errorf("the server sends the %s token %q, which the client does not know: it will resolve to a default", kind, name)
		}
	}

	inServer := map[string]bool{}
	for _, name := range used {
		inServer[name] = true
	}
	for _, name := range declared {
		if !inServer[name] {
			t.Errorf("the client declares the %s token %q and nothing sends it: either the screen that needed it is missing, or the name is", kind, name)
		}
	}
}
