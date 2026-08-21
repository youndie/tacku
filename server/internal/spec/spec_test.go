package spec_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/youndie/tacku/server/internal/spec"
)

// specDir walks up from the test's working directory to the repository root, so the test does not
// depend on where `go test` was invoked from.
func specDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "spec")
		if _, err := os.Stat(filepath.Join(candidate, spec.ProfileFileName)); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("no spec directory above the working directory; run client/gradlew :spec-gen:test with TACKU_SPEC_RECORD=true")
	return ""
}

func load(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := spec.Load(specDir(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLoadReadsEveryModuleTheProfileNames(t *testing.T) {
	s := load(t)

	if len(s.Modules) != 11 {
		t.Fatalf("profile names %d modules, want 11 (ten of the toolkit plus form-standard)", len(s.Modules))
	}
	if !slices.Contains(s.Modules, "form-standard") {
		t.Error("form-standard missing: it is the module the toolkit's own spec set leaves out, so it is the one that has to be here")
	}
	// One file per module, plus the profile itself.
	if len(s.Schemas) != len(s.Modules)+1 {
		t.Errorf("loaded %d schema files for %d modules plus a profile", len(s.Schemas), len(s.Modules))
	}
}

func TestProfileCarriesEveryHierarchy(t *testing.T) {
	s := load(t)

	want := map[string]int{
		"KompotComponent":     15,
		"KompotAction":        10,
		"FormFieldDefinition": 5,
		"ValidationRule":      4,
		"FieldValue":          4,
		"FormCondition":       2,
	}
	for hierarchy, count := range want {
		got := s.Types(hierarchy)
		if len(got) != count {
			t.Errorf("%s: %d types %v, want %d", hierarchy, len(got), got, count)
		}
	}
}

// The point of a profile is that something falls outside it. A test that only asserts membership
// passes just as happily against a set that accepts everything, so both directions are checked —
// and the negative case uses a name shaped like a real one rather than obvious junk.
func TestProfileIsClosed(t *testing.T) {
	s := load(t)

	if !s.Declares("FormFieldDefinition", "text_field") {
		t.Error("text_field is declared by this build and must be in the profile")
	}
	if s.Declares("FormFieldDefinition", "date_field") {
		t.Error("date_field is not part of this build; the profile accepted a type nobody declared")
	}
	if s.Declares("KompotComponent", "tabs") {
		t.Error("tabs is not in the vocabulary; the profile accepted a type nobody declared")
	}
}

// Types distinguishes "no such hierarchy" from "a hierarchy with nothing in it". Without this the
// name of a hierarchy could be misspelled anywhere above and every membership check would quietly
// answer false.
func TestUnknownHierarchyIsNotAnEmptySet(t *testing.T) {
	s := load(t)

	if s.Types("KompotComponents") != nil {
		t.Error("a misspelled hierarchy must return nil, not an empty set")
	}
	if s.Types("KompotComponent") == nil {
		t.Error("a real hierarchy must return its types")
	}
}

func TestLoadFailsLoudlyWithoutADirectory(t *testing.T) {
	t.Setenv(spec.DirEnv, "")
	if _, err := spec.Load(""); err == nil {
		t.Error("loading with no directory must fail rather than return an empty spec")
	}
	if _, err := spec.Load(t.TempDir()); err == nil {
		t.Error("loading an empty directory must fail rather than return an empty spec")
	}
}
