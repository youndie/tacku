package spec_test

import (
	"strings"
	"testing"
)

// A form envelope carrying a field type nobody declared.
//
// Shaped like a real one on purpose: it satisfies kompot-forms.schema.json#/$defs/KompotFormResponse,
// because that path reaches FormFieldDefinition through its OPEN base — "an object with a type"
// (SPEC.md §2.1) — and the closed list lives only in the profile. This body is what the schema check
// alone lets through, and what the walk exists to catch.
const formWithAnUndeclaredField = `{
  "schema": {"formId": "task_create", "fields": [
    {"type": "text_field", "fieldId": "title", "rules": [{"type": "required", "errorMessage": "no"}]},
    {"type": "date_field", "fieldId": "due", "rules": []}
  ]},
  "screen": {"type": "column", "id": "root", "children": [
    {"type": "text_input", "id": "field-title", "fieldId": "title", "label": "Title"}
  ]}
}`

const formResponse = "kompot-forms.schema.json#/$defs/KompotFormResponse"

func TestTheWalkFindsAFieldTypeTheProfileDoesNotDeclare(t *testing.T) {
	s := load(t)

	result, err := s.Scan(formResponse, []byte(formWithAnUndeclaredField))
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Undeclared) != 1 {
		t.Fatalf("found %v, want exactly the invented field", result.Undeclared)
	}
	found := result.Undeclared[0]
	if found.WireType != "date_field" || found.Hierarchy != "FormFieldDefinition" {
		t.Errorf("found %+v, want date_field in FormFieldDefinition", found)
	}
	if found.Degrades {
		t.Error("a form field was reported as degrading; §2.2 says an unknown one costs the whole response")
	}
	if !strings.Contains(found.Path, "fields[1]") {
		t.Errorf("the finding points at %q, which does not say where in the body to look", found.Path)
	}
}

// The other direction. A walk that reported everything would pass the test above just as happily.
func TestTheWalkAcceptsAResponseMadeOfDeclaredTypesOnly(t *testing.T) {
	s := load(t)

	clean := strings.Replace(formWithAnUndeclaredField,
		`{"type": "date_field", "fieldId": "due", "rules": []}`,
		`{"type": "text_field", "fieldId": "due", "rules": []}`, 1)

	result, err := s.Scan(formResponse, []byte(clean))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Undeclared) != 0 {
		t.Errorf("a response made of declared types was reported: %v", result.Undeclared)
	}
	if result.Visited["FormFieldDefinition"] != 2 {
		t.Errorf("the walk reached %d field definitions, want 2", result.Visited["FormFieldDefinition"])
	}
	if result.Visited["ValidationRule"] != 1 {
		t.Errorf("the walk reached %d rules, want 1", result.Visited["ValidationRule"])
	}
	if result.Visited["KompotComponent"] != 2 {
		t.Errorf("the walk reached %d components, want 2", result.Visited["KompotComponent"])
	}
}

// The two halves of §2 must be told apart, because the same finding costs a placeholder in one and
// an empty screen in the other. A walk that reported both the same way would leave a reader with no
// way to know which one they are holding.
func TestTheWalkSeparatesTheHalvesOfTheProtocol(t *testing.T) {
	s := load(t)

	invented := strings.Replace(formWithAnUndeclaredField,
		`{"type": "text_input", "id": "field-title", "fieldId": "title", "label": "Title"}`,
		`{"type": "tabs", "id": "field-title"}`, 1)

	result, err := s.Scan(formResponse, []byte(invented))
	if err != nil {
		t.Fatal(err)
	}

	costs := map[string]bool{}
	for _, found := range result.Undeclared {
		costs[found.WireType] = found.Degrades
	}
	if degrades, found := costs["tabs"]; !found || !degrades {
		t.Errorf("an undeclared component was reported as %v; §2.1 gives it a fallback", costs)
	}
	if degrades, found := costs["date_field"]; !found || degrades {
		t.Errorf("an undeclared field was reported as %v; §2.2 gives it none", costs)
	}
}

// The dangerous hierarchy is not confined to forms.
//
// `perform` carries a map of FieldValue in its payload, and `perform` is an action — it rides in a
// screen tree, on a button of a card. A value type the client does not know there fails the parse of
// a whole screen, on an endpoint that has no form anywhere in it. A check that looked only at form
// endpoints would never see this.
func TestTheWalkReachesValuesCarriedInsideAScreen(t *testing.T) {
	s := load(t)

	screen := `{"type": "column", "id": "root", "children": [
	  {"type": "button", "id": "move", "text": "Move", "action":
	    {"type": "perform", "url": "/submit/move", "payload":
	      {"task": {"type": "text_value", "text": "TAC-1"},
	       "due": {"type": "date_value", "date": "2026-08-29"}}}}
	]}`

	result, err := s.Scan("kompot-core.schema.json#/$defs/KompotComponent", []byte(screen))
	if err != nil {
		t.Fatal(err)
	}

	if result.Visited["FieldValue"] != 2 {
		t.Fatalf("the walk reached %d values inside the screen, want 2: it never got into the payload",
			result.Visited["FieldValue"])
	}
	if len(result.Undeclared) != 1 || result.Undeclared[0].WireType != "date_value" {
		t.Fatalf("found %v, want the invented value", result.Undeclared)
	}
	if result.Undeclared[0].Degrades {
		t.Error("a value inside a screen was reported as degrading; it is the button's payload that breaks the screen")
	}
}

// The closed hierarchy of the protocol (§2.3) is closed by the module schema rather than by the
// profile, so it is reached through a different door and would be the first thing to break silently
// if the walk only ever asked the profile.
func TestTheWalkChecksModifiersAgainstTheProtocolsOwnList(t *testing.T) {
	s := load(t)

	tree := `{"type": "column", "id": "root", "modifiers": [{"type": "blur", "radius": 4}]}`

	result, err := s.Scan("kompot-core.schema.json#/$defs/KompotComponent", []byte(tree))
	if err != nil {
		t.Fatal(err)
	}
	if result.Visited["KompotModifierNode"] != 1 {
		t.Fatalf("the walk reached %d modifiers, want 1", result.Visited["KompotModifierNode"])
	}
	if len(result.Undeclared) != 1 || result.Undeclared[0].WireType != "blur" {
		t.Fatalf("found %v, want the invented modifier", result.Undeclared)
	}
}

func TestScanFailsLoudlyOnANonsenseReference(t *testing.T) {
	s := load(t)

	if _, err := s.Scan("kompot-core.schema.json#/$defs/NoSuchThing", []byte(`{}`)); err == nil {
		t.Error("a reference to a definition nobody declares must fail rather than report nothing")
	}
	if _, err := s.Scan(formResponse, []byte(`not json`)); err == nil {
		t.Error("a body that is not JSON must fail rather than report nothing")
	}
}

// The list of hierarchies with no fallback is read out of the contract, not typed in. A build that
// took it from memory would keep saying "four" after a release that added a fifth, and the new one
// would arrive with nothing looking at it.
func TestTheDangerousHierarchiesAreReadOutOfTheSchemas(t *testing.T) {
	s := load(t)

	dangerous := map[string]bool{}
	for _, name := range s.NonDegrading() {
		dangerous[name] = true
	}

	// §2.2 names four, §2.3 closes a fifth, and the wizard's transitions are the sixth — no
	// fallback anywhere among them.
	for _, name := range []string{"FormFieldDefinition", "ValidationRule", "FieldValue",
		"FormCondition", "KompotModifierNode", "WizardTransition"} {
		if !dangerous[name] {
			t.Errorf("%s has no fallback and is missing from the list", name)
		}
	}
	// And the other direction, which is the half that makes the list mean anything: the two
	// hierarchies that do degrade must not be in it, or every distinction this file draws collapses.
	for _, name := range []string{"KompotComponent", "KompotAction"} {
		if dangerous[name] {
			t.Errorf("%s degrades (§2.1) and must not be counted among the dangerous ones", name)
		}
	}
}
