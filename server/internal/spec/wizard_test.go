package spec_test

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/spec"
)

// The words a control is named with, in every spelling this contract uses for them: `button.text`,
// `text_input.label`, `read_only_field.helperText`, `screen_route.title`. Read as a list of
// spellings rather than of properties, so that a field named under any of them counts as words.
var words = []string{"label", "title", "text", "caption"}

// The chrome of a scenario carries one word from the server, and exactly one (B-31).
//
// Next, Back and Finish are drawn by the client out of `stepIndex`, `totalSteps` and `canGoBack`,
// so until kompot 0.21 the finishing button read the same in every scenario of every build —
// tolerable under "New task" and not under a step that deletes a board. `finishLabel` was asked for
// on those grounds and arrived.
//
// The previous version of this test asserted the opposite and was written to go red from either
// side: it named the eight properties the type had and failed the day a ninth appeared, with a
// message saying the gap was closed. It fired. This is what replaced it, and the shape of the check
// is the same — it fails if the field disappears and it fails if the chrome grows words the server
// is not using.
func TestTheScenarioNamesItsOwnFinishButton(t *testing.T) {
	s := load(t)

	properties := propertiesOf(t, s, "KompotComponent", "wizard_screen")

	if !slices.Contains(properties, "finishLabel") {
		t.Fatalf("wizard_screen carries %v and no finishLabel — the field this product relies on is gone", properties)
	}

	// Only the finishing one. Back and Next move rather than commit, and the cost of their wording
	// is not the same — a label for each would be three decisions where one was needed.
	named := []string{}
	for _, property := range properties {
		for _, word := range words {
			if strings.Contains(strings.ToLower(property), word) {
				named = append(named, property)
			}
		}
	}
	if !slices.Equal(named, []string{"finishLabel"}) {
		t.Errorf("the chrome carries %v, want only finishLabel — a word the server does not fill is a word somebody else chose", named)
	}
}

// The other direction, without which the test above proves nothing.
//
// A detector that never matches anything reports "no words here" about every component in the
// contract, including the ones made of words. This is the same detector run over a control the
// server does place itself — and it must find them.
func TestTheSameDetectorFindsTheWordsOnAControlTheServerPlaces(t *testing.T) {
	s := load(t)

	found := 0
	for _, wireType := range []string{"button", "text", "text_input", "select_input"} {
		for _, property := range propertiesOf(t, s, "KompotComponent", wireType) {
			for _, word := range words {
				if strings.Contains(strings.ToLower(property), word) {
					found++
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no control in the profile was found to carry words, so the detector measures itself")
	}
}

// An extra key on a wizard screen is valid, which is what makes carrying the label there look
// available at all.
//
// The half a schema cannot show is what happens after validation: the published type has eight
// properties and no room for a ninth, so the key travels, validates and arrives nowhere. That half
// is checked on the Kotlin side, in WizardChromeTest, against the artefact rather than the file.
func TestAnExtraKeyOnAWizardScreenIsValid(t *testing.T) {
	s := load(t)

	var parsed struct {
		Defs map[string]struct {
			Additional json.RawMessage `json:"additionalProperties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(s.Schemas["kompot-wizard.schema.json"], &parsed); err != nil {
		t.Fatal(err)
	}
	definition, ok := parsed.Defs["KompotComponentWizardScreen"]
	if !ok {
		t.Fatal("kompot-wizard.schema.json declares no KompotComponentWizardScreen")
	}
	if strings.TrimSpace(string(definition.Additional)) != "true" {
		t.Errorf("additionalProperties is %s; an extra key would be refused, and the route is not even open",
			definition.Additional)
	}
}

// propertiesOf resolves a wire type through the profile the way a reader of the contract would —
// the closed list is there and nowhere else — and returns the property names of what it points at,
// sorted.
func propertiesOf(t *testing.T, s *spec.Spec, hierarchy, wireType string) []string {
	t.Helper()

	var profile struct {
		Defs map[string]struct {
			Discriminator struct {
				Mapping map[string]string `json:"mapping"`
			} `json:"discriminator"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(s.Profile, &profile); err != nil {
		t.Fatal(err)
	}
	reference, ok := profile.Defs[hierarchy].Discriminator.Mapping[wireType]
	if !ok {
		t.Fatalf("the profile declares no %s in %s", wireType, hierarchy)
	}

	const defs = "#/$defs/"
	index := strings.Index(reference, defs)
	if index < 0 {
		t.Fatalf("%s points at %q, which does not name a definition", wireType, reference)
	}
	file, name := reference[:index], reference[index+len(defs):]
	if file == "" {
		file = spec.ProfileFileName
	}

	var document struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(s.Schemas[file], &document); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	definition, ok := document.Defs[name]
	if !ok {
		t.Fatalf("%s declares no %s", file, name)
	}
	if len(definition.Properties) == 0 {
		t.Fatalf("%s carries no properties at all, so nothing was looked at", name)
	}

	names := make([]string, 0, len(definition.Properties))
	for property := range definition.Properties {
		names = append(names, property)
	}
	sort.Strings(names)
	return names
}
