package render_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/spec"
)

// Every wire type this package is able to build, against the closed profile of this build.
//
// This is not the check the httpsrv package already makes over responses. That one sees the types
// the current screens happen to reach; this one sees the types the package can produce at all. A
// constructor written before it is wired into a screen — which is exactly how a new component gets
// added — is invisible to the first and caught by the second.
//
// It stands in front of a failure that is quiet on both sides of the wire. KompotComponent is
// declared OPEN in kompot-core and degrades to a placeholder, so an invented component costs no
// error anywhere: the client draws a box, the rest of the screen renders, and the meaning that node
// carried is simply absent. The profile is the only artefact that says which types this build may
// send, and it is the artefact the conformance kit reads (see docs/research/questions.md, Q-24).
func TestEveryTypeThisPackageBuildsIsDeclared(t *testing.T) {
	declared := declaredTypes(t)
	built := builtTypes(t)

	if len(declared) == 0 {
		t.Fatal("the profile declares no wire type at all, so this check has nothing to compare against")
	}
	if len(built) == 0 {
		t.Fatal("no wire type literal was found in the package, so this check looked at nothing")
	}
	t.Logf("%d wire types built here, %d declared by this build", len(built), len(declared))

	for _, wireType := range built {
		if !declared[wireType] {
			t.Errorf("this package builds the wire type %q, which no hierarchy of this build declares: "+
				"the client degrades it to a placeholder and says nothing, and the conformance kit reads the same profile",
				wireType)
		}
	}
}

// declaredTypes is every discriminator value of every closed hierarchy of this build: the profile
// carries the hierarchies an application composes with, kompot-core carries the modifier chain,
// which the profile does not repeat.
func declaredTypes(t *testing.T) map[string]bool {
	t.Helper()

	loaded, err := spec.Load(specDir(t))
	if err != nil {
		t.Fatal(err)
	}

	declared := map[string]bool{}
	for _, document := range []json.RawMessage{loaded.Profile, loaded.Schemas[spec.FileNameFor("kompot-core")]} {
		var parsed struct {
			Defs map[string]struct {
				Discriminator struct {
					Mapping map[string]string `json:"mapping"`
				} `json:"discriminator"`
			} `json:"$defs"`
		}
		if err := json.Unmarshal(document, &parsed); err != nil {
			t.Fatal(err)
		}
		for _, def := range parsed.Defs {
			for wireType := range def.Discriminator.Mapping {
				declared[wireType] = true
			}
		}
	}
	return declared
}

// builtTypes reads the package's own source rather than its output: a constructor nothing calls yet
// is the case this check exists for, and it produces no output to read.
func builtTypes(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]bool{}
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			pair, ok := node.(*ast.KeyValueExpr)
			if !ok || !namesTheDiscriminator(pair.Key) {
				return true
			}
			if value, ok := stringValue(pair.Value); ok {
				found[value] = true
			}
			return true
		})
	}

	types := make([]string, 0, len(found))
	for wireType := range found {
		types = append(types, wireType)
	}
	sort.Strings(types)
	return types
}

// The discriminator is written two ways in this package: a struct field named Type, tagged
// json:"type", and the one payload built as a map, where the key is the string itself.
func namesTheDiscriminator(key ast.Expr) bool {
	if ident, ok := key.(*ast.Ident); ok {
		return ident.Name == "Type"
	}
	value, ok := stringValue(key)
	return ok && value == "type"
}

func stringValue(expr ast.Expr) (string, bool) {
	basic, ok := expr.(*ast.BasicLit)
	if !ok || basic.Kind != token.STRING {
		return "", false
	}
	unquoted, err := strconv.Unquote(basic.Value)
	if err != nil {
		return "", false
	}
	return unquoted, true
}

func specDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		candidate := filepath.Join(dir, "spec")
		if _, err := os.Stat(filepath.Join(candidate, spec.ProfileFileName)); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("no spec directory above the working directory")
	return ""
}
