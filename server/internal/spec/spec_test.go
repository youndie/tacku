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

	// Thirteen since kompot 0.11: the toolkit now describes form-standard and kompot-theme itself,
	// and kompot-commands arrived with the action that acts on one item of a list. Both were gaps
	// this project reported, so the number moving is the report landing rather than a surprise.
	if len(s.Modules) != 13 {
		t.Fatalf("profile names %d modules, want 13", len(s.Modules))
	}
	for _, module := range []string{"form-standard", "kompot-commands"} {
		if !slices.Contains(s.Modules, module) {
			t.Errorf("%s is missing from the profile", module)
		}
	}
	// One file per module, plus the profile itself.
	if len(s.Schemas) != len(s.Modules)+1 {
		t.Errorf("loaded %d schema files for %d modules plus a profile", len(s.Schemas), len(s.Modules))
	}
}

func TestProfileCarriesEveryHierarchy(t *testing.T) {
	s := load(t)

	// Seventeen and six rather than fifteen and five: this deployment declares three wire types of
	// its own, and the profile counts them alongside the toolkit's because that is what a profile is
	// for — what THIS BUILD serves, not what the protocol defines. Two of the three are components,
	// which is the cheap half of extending: an unfamiliar one costs a node rather than a response.
	want := map[string]int{
		// Sixteen. It was seventeen while this deployment carried a component of its own for
		// multiline text; kompot 0.21 put the same thing on `text_input` as a flag, so the type
		// went away and the count came down with it — a number that moves when a decision moves.
		"KompotComponent": 16,
		// Twelve. `open_url` arrived in kompot 0.32, and it arrived because this project asked for
		// it: the vocabulary had no way to leave the application, so a card standing for a file in
		// another repository could not lead to it (Q-72, kompot#55). A count that moves when the
		// protocol moves is the point of writing it down.
		"KompotAction":        12,
		"FormFieldDefinition": 6,
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
	// `date_field` used to stand here, chosen because it was shaped like a real name — and then it
	// became one. A negative fixture picked for plausibility is a fixture that can stop being
	// negative, which is worth knowing before the day it does so quietly.
	if !s.Declares("FormFieldDefinition", "date_field") {
		t.Error("date_field is this deployment's own field type and must be declared as an extension")
	}
	if s.Declares("FormFieldDefinition", "colour_field") {
		t.Error("colour_field is not part of this build; the profile accepted a type nobody declared")
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
