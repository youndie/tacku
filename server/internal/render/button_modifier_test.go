package render_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// drawnByTheClient is every modifier constructor the client's own button renderer implements.
//
// The client draws `button` itself (client/app/.../ButtonRenderer.kt) because the design needs a
// square control whose whole painted area is clickable, and Material's button is neither. That
// choice moves a cost here: the toolkit's renderer understood the whole modifier chain, and ours
// understands two links of it. A third one added to a button on this side would be dropped on that
// side — the button would still draw, still work, and simply be the wrong size or the wrong colour.
var drawnByTheClient = map[string]bool{
	"PaddingXY":  true,
	"Padding":    true,
	"PaddingAll": true,
	"Background": true,
}

// Every modifier this server puts on a button, against what the client's button renderer draws.
//
// Read from the source rather than from a rendered screen on purpose: a button reached by no test
// screen is exactly the one whose modifier nobody notices, and the failure has no error anywhere —
// not a refusal, not a placeholder, not a log line on the wire. It is a control that looks slightly
// wrong.
func TestEveryButtonModifierIsOneTheClientDraws(t *testing.T) {
	found := 0
	seen := map[string]bool{}

	for _, dir := range []string{".", filepath.Join("..", "httpsrv")} {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatal(err)
		}

		for _, pkg := range pkgs {
			for name, file := range pkg.Files {
				ast.Inspect(file, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok || calleeName(call.Fun) != "Button" {
						return true
					}
					found++
					// id, label, action, then the modifiers.
					for _, arg := range call.Args[3:] {
						modifier, ok := arg.(*ast.CallExpr)
						if !ok {
							continue
						}
						kind := calleeName(modifier.Fun)
						seen[kind] = true
						if !drawnByTheClient[kind] {
							t.Errorf("%s: a button carries %s, which the client's own button renderer does not draw: "+
								"the modifier is dropped and the control simply looks wrong",
								filepath.Base(name), kind)
						}
					}
					return true
				})
			}
		}
	}

	// A walk that found no button would pass in silence, and so would one that found buttons and
	// no modifier on any of them.
	if found == 0 {
		t.Fatal("no button call was found at all, so this check looked at nothing")
	}
	if len(seen) == 0 {
		t.Fatalf("%d buttons found and not one modifier among them: this check proved nothing", found)
	}

	kinds := make([]string, 0, len(seen))
	for kind := range seen {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	t.Logf("%d buttons, modifiers used: %v", found, kinds)
}

func calleeName(fun ast.Expr) string {
	switch called := fun.(type) {
	case *ast.Ident:
		return called.Name
	case *ast.SelectorExpr:
		return called.Sel.Name
	}
	return ""
}
