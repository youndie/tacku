package render_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// Where a phrase is allowed to stand.
//
// The decision is written at the top of copy.go: text about data belongs to that file, text about a
// control stays at the element that shows it. This is the half that survives the person who read it.
// The migration was the cheap part — a phrase written tomorrow by somebody who never opened copy.go
// is the failure the task was actually about, and a convention does not catch that.
//
// The rules are deliberately narrower than the decision, because these are the parts a machine can
// hold:
//
//  1. Nothing is assembled outside copy.go. A literal carrying a space may not be glued to a value
//     with `+`, and may not be the format of an fmt call. An assembled phrase is a grammar, and a
//     grammar is what the next handler instantiates in its own words. Gluing two constants is not
//     assembly and is left alone: "POST "+LoginPath composes a route, not a sentence.
//  2. Nothing is bound outside copy.go. Prose may stand as an argument of the call that renders it
//     and nowhere else — not assigned to a variable, not returned, not declared. A phrase that needs
//     a name or a branch is a phrase chosen by data, and choosing by data is copy.go's job.
//  3. Time is formatted only in copy.go. A date is text about data, and two layouts on two screens
//     is the smallest version of the same drift.
//
// One shape is exempt: a `Bearer …` challenge. It is a protocol header whose grammar belongs to
// RFC 6750, read by a program and by whoever holds the token, and never by somebody using the
// product. The exemption is on the value rather than on the file, so a sentence written beside it
// is still caught — an exemption the width of a file is how a guard stops guarding.
//
// What the rules do not catch is written down rather than pretended away: a stand-in like
// "No description yet." is chosen by data without being assembled, and only the discipline of the
// decision puts it in copy.go. The guard holds the line it can hold and says which line that is.
func TestNoPhraseIsWrittenOutsideItsPlace(t *testing.T) {
	files, spaced, prose := walkSources(t, func(t *testing.T, file *sourceFile) {
		t.Helper()

		for _, found := range file.literals {
			if strings.Contains(found.value, " ") && found.gluedToValue {
				t.Errorf("%s:%d assembles interface text out of %q. "+
					"An assembled phrase belongs in copy.go: it is a grammar, and the next handler "+
					"will instantiate it in its own words",
					file.name, found.line, found.value)
			}
			if strings.Contains(found.value, " ") && found.isFormat {
				t.Errorf("%s:%d formats interface text from %q. "+
					"An assembled phrase belongs in copy.go: it is a grammar, and the next handler "+
					"will instantiate it in its own words",
					file.name, found.line, found.value)
			}
			if isProse(found.value) && found.bound {
				t.Errorf("%s:%d gives the phrase %q a name of its own. "+
					"A phrase that has to be named or branched on is chosen by data, and text about "+
					"data lives in copy.go; a caption stands where it is shown",
					file.name, found.line, found.value)
			}
		}

		for _, line := range file.timeFormats {
			t.Errorf("%s:%d formats a time. Dates are text about data and are written in copy.go, "+
				"which is what keeps one day from being spelled two ways on two screens",
				file.name, line)
		}
	})

	// A check that found nothing to look at proves nothing. These are the numbers the run actually
	// saw, and the floors are far below them: they fail when the walk stops reaching the sources,
	// not when somebody adds a screen.
	t.Logf("scanned %d files outside copy.go: %d literals carrying a space, %d of them prose",
		files, spaced, prose)
	if files < 5 {
		t.Fatalf("the walk read %d source files, so it was not looking at the server", files)
	}
	if prose < 20 {
		t.Fatalf("the walk found %d phrases in the screens; that is not this product", prose)
	}
}

// The detector, pointed at the file it is meant to permit.
//
// Every rule above is a negative, and a negative passes on a scanner that sees nothing. copy.go is
// full of exactly what the rules forbid elsewhere, so it is the fixture that proves the scanner
// still recognises the thing it is banning.
func TestTheGuardStillRecognisesWhatItBans(t *testing.T) {
	file := readSource(t, filepath.Join(".", "copy.go"))

	// Counted per rule and not together: a single total stays comfortably above a floor while one
	// of the three rules quietly stops seeing anything at all.
	glued, formatted, bound, formats := 0, 0, 0, len(file.timeFormats)
	for _, found := range file.literals {
		if strings.Contains(found.value, " ") && found.gluedToValue {
			glued++
		}
		if strings.Contains(found.value, " ") && found.isFormat {
			formatted++
		}
		if isProse(found.value) && found.bound {
			bound++
		}
	}

	t.Logf("copy.go: %d phrases glued to a value, %d formatted, %d bound, %d time formats",
		glued, formatted, bound, formats)
	if glued < 3 {
		t.Errorf("the scanner sees %d phrases glued to a value in copy.go, where the meta lines are "+
			"built out of separators; that rule is passing on a blind scan", glued)
	}
	if formatted < 10 {
		t.Errorf("the scanner sees %d formatted phrases in copy.go; the journal alone has more, "+
			"so that rule is passing on a blind scan", formatted)
	}
	if bound < 1 {
		t.Error("the scanner sees no bound phrase in copy.go, where the stand-ins are returned " +
			"outright; the binding rule is passing on a blind scan")
	}
	if formats < 1 {
		t.Error("the scanner sees no time formatting in copy.go, where every date is written; " +
			"the time rule is passing on a blind scan")
	}
}

// homes are the two files allowed to hold interface text that is not standing at its element.
//
// copy.go by the decision. routes.go because a destination is named by the row that declares it:
// the graph carries a title for the client, and holding that title anywhere else is how this
// repository ended up with "Board" in the graph and "Boards" on the button next to it.
var homes = map[string]bool{
	filepath.Join(".", "copy.go"):   true,
	filepath.Join(".", "routes.go"): true,
}

// tooling names the files whose text is addressed to a machine and to whoever reads the API rather
// than to a person using the product: the OpenAPI description exists for the conformance kit, and
// its response descriptions answer to that document rather than to a screen.
var tooling = map[string]bool{
	filepath.Join("..", "httpsrv", "openapi.go"): true,
}

// surfaces are the packages that put words in front of a person: the renderer and the HTTP layer
// that drives it. mcpsrv is deliberately absent — its text is addressed to a model and belongs to
// the tool contract, and one owner over two vocabularies with opposite requirements would be a rule
// nobody could follow.
var surfaces = []string{".", filepath.Join("..", "httpsrv")}

type literal struct {
	value        string
	line         int
	gluedToValue bool
	isFormat     bool
	bound        bool
}

type sourceFile struct {
	name        string
	literals    []literal
	timeFormats []int
}

func walkSources(t *testing.T, check func(*testing.T, *sourceFile)) (files, spaced, prose int) {
	t.Helper()

	for _, dir := range surfaces {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			if homes[path] || tooling[path] {
				continue
			}

			file := readSource(t, path)
			files++
			for _, found := range file.literals {
				if strings.Contains(found.value, " ") {
					spaced++
				}
				if isProse(found.value) {
					prose++
				}
			}
			check(t, file)
		}
	}
	return files, spaced, prose
}

// readSource collects, for every string literal, the two things the rules ask about: whether the
// phrase is combined with something, and whether it is bound rather than shown.
func readSource(t *testing.T, path string) *sourceFile {
	t.Helper()

	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	file := &sourceFile{name: path}
	var stack []ast.Node

	ast.Inspect(parsed, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		defer func() { stack = append(stack, node) }()

		if call, ok := node.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Format" {
				file.timeFormats = append(file.timeFormats, set.Position(call.Pos()).Line)
			}
		}

		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		// An authentication challenge is a protocol header, not interface text: it is read by a
		// program and by whoever is holding the token, never by somebody using the product, and its
		// grammar is RFC 6750's rather than ours. Skipped by the shape of the value and not by the
		// file it sits in, so a sentence written next to it is still caught.
		if strings.HasPrefix(value, "Bearer ") {
			return true
		}

		file.literals = append(file.literals, literal{
			value:        value,
			line:         set.Position(lit.Pos()).Line,
			gluedToValue: gluedToValue(stack),
			isFormat:     formatOf(stack, lit),
			bound:        boundNotShown(stack),
		})
		return true
	})

	return file
}

// gluedToValue reports whether the literal is joined by `+` to something that is not itself a
// literal or a plain name.
//
// The distinction is what separates a sentence from a route pattern. `" · " + StatusName(...)` and
// `"← " + string(board)` put words around a value and are copy's business; `"POST " + LoginPath`
// joins two constants and is not text at all. A separator carries no letters of its own, so the
// prose test cannot see it — this is the rule that does.
func gluedToValue(stack []ast.Node) bool {
	var outermost ast.Expr
	for i := len(stack) - 1; i >= 0; i-- {
		binary, ok := stack[i].(*ast.BinaryExpr)
		if !ok || binary.Op != token.ADD {
			break
		}
		outermost = binary
	}
	if outermost == nil {
		return false
	}

	glued := false
	ast.Inspect(outermost, func(node ast.Node) bool {
		switch node.(type) {
		case nil, *ast.BinaryExpr, *ast.ParenExpr, *ast.Ident, *ast.BasicLit:
			return true
		}
		glued = true
		return false
	})
	return glued
}

// formatOf reports whether the literal is the format string of an fmt call.
//
// fmt.Errorf and errors.New are not asked about: an error value is addressed to whoever reads a log
// or a stack, and calling that interface copy would put an owner on text no person ever sees.
func formatOf(stack []ast.Node, lit *ast.BasicLit) bool {
	if len(stack) == 0 {
		return false
	}
	call, ok := stack[len(stack)-1].(*ast.CallExpr)
	if !ok || len(call.Args) == 0 || call.Args[0] != ast.Expr(lit) {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "fmt" || selector.Sel.Name == "Errorf" {
		return false
	}
	return true
}

// boundNotShown reports whether the literal is given a name instead of standing where it is shown.
//
// The nearest enclosing binder wins over the nearest call: a phrase inside a table that is itself an
// argument — the body of an error answer, say — is shown, while the same phrase in a variable, a
// return or a declaration has been given a life of its own.
func boundNotShown(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.CallExpr:
			return false
		case *ast.AssignStmt, *ast.ReturnStmt, *ast.ValueSpec:
			return true
		}
	}
	return false
}

// isProse is the narrow test: a letter, a space, a letter. It passes over separators (" · "), over
// arrows ("← ") and over wire tokens with a trailing space ("Bearer "), which the concatenation rule
// catches instead if they are ever used to build a sentence.
func isProse(value string) bool {
	seenLetter, seenSpace := false, false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r):
			if seenLetter && seenSpace {
				return true
			}
			seenLetter = true
		case r == ' ':
			if seenLetter {
				seenSpace = true
			}
		}
	}
	return false
}
