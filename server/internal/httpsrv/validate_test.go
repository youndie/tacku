package httpsrv_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/youndie/tacku/server/internal/spec"
)

// specDir walks up to the repository root, so the test does not depend on where go test was called
// from — the same helper the spec package's own tests use.
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

func validator(t *testing.T) *spec.Validator {
	t.Helper()

	loaded, err := spec.Load(specDir(t))
	if err != nil {
		t.Fatal(err)
	}
	v, err := spec.NewValidator(loaded)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Every response this server sends, against the schema of this build.
//
// The mitigation for the risk the project carries by construction: the form hierarchies do not
// degrade, so a wire type the client does not know costs a whole screen rather than one node, and
// Go has no compiler to stop a wrong string. This is the only thing standing in that gap.
func TestEveryResponseSatisfiesTheSchema(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)
	v := validator(t)

	cases := []struct {
		path      string
		reference string
	}{
		{"/screens/catch-up", spec.Profile("KompotComponent")},
		{"/screens/board", spec.Profile("KompotComponent")},
		{"/forms/new-task", spec.In("kompot-forms", "KompotFormResponse")},
		{"/pages/changes", spec.In("kompot-standard", "KompotPageResponse")},
		{"/graph", spec.In("kompot-navigation", "NavigationGraph")},
	}

	for _, c := range cases {
		response, body := r.get(t, c.path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Errorf("%s answered %d: %s", c.path, response.StatusCode, body)
			continue
		}
		if err := v.Validate(c.reference, body); err != nil {
			t.Errorf("%s: %v", c.path, err)
		}
	}
}

// Without this the check above would pass against a schema that accepts anything, and the whole
// mitigation would be decoration. The invented types are shaped like real ones on purpose — obvious
// junk would be refused by something coarser than the profile.
func TestTheValidatorRefusesTypesNobodyDeclared(t *testing.T) {
	v := validator(t)

	refused := map[string]string{
		"a component outside the profile": `{"type":"tabs","id":"x"}`,
		"a plausible field type":          `{"type":"date_field","fieldId":"due"}`,
		"a plausible value type":          `{"type":"date_value","date":"2026-08-29"}`,
	}
	references := map[string]string{
		"a component outside the profile": spec.Profile("KompotComponent"),
		"a plausible field type":          spec.Profile("FormFieldDefinition"),
		"a plausible value type":          spec.Profile("FieldValue"),
	}

	for name, body := range refused {
		if err := v.Validate(references[name], []byte(body)); err == nil {
			t.Errorf("%s was accepted; the profile is not closed and the check above proves nothing", name)
		}
	}

	// And the other direction, so a validator that refuses everything cannot pass this file either.
	accepted := map[string]string{
		spec.Profile("KompotComponent"):     `{"type":"text","id":"x","text":"hello"}`,
		spec.Profile("FormFieldDefinition"): `{"type":"text_field","fieldId":"title","rules":[]}`,
		spec.Profile("FieldValue"):          `{"type":"text_value","text":"hello"}`,
	}
	for reference, body := range accepted {
		if err := v.Validate(reference, []byte(body)); err != nil {
			t.Errorf("a declared type was refused by %s: %v", reference, err)
		}
	}
}

// The rule the whole package exists to keep: every input names a field the schema declares.
func TestEveryInputNamesADeclaredField(t *testing.T) {
	r := newResource(t)
	r.seed(t)

	_, body := r.get(t, "/forms/new-task", r.reader(t), "")

	var form struct {
		Schema struct {
			Fields []struct {
				FieldID string `json:"fieldId"`
			} `json:"fields"`
		} `json:"schema"`
		Screen map[string]any `json:"screen"`
	}
	if err := json.Unmarshal(body, &form); err != nil {
		t.Fatal(err)
	}

	declared := map[string]bool{}
	for _, field := range form.Schema.Fields {
		declared[field.FieldID] = true
	}
	if len(declared) == 0 {
		t.Fatal("the form declares no fields, so this check has nothing to look at")
	}

	used := map[string]bool{}
	collectFieldIDs(form.Screen, used)
	if len(used) == 0 {
		t.Fatal("the tree holds no inputs, so this check has nothing to look at")
	}

	for fieldID := range used {
		if !declared[fieldID] {
			t.Errorf("an input names %q, which the schema does not declare: it has no validation and no place in the payload", fieldID)
		}
	}
	for fieldID := range declared {
		if !used[fieldID] {
			t.Errorf("the schema declares %q and no input shows it, so nothing can fill it", fieldID)
		}
	}
}

func collectFieldIDs(node any, into map[string]bool) {
	value, ok := node.(map[string]any)
	if !ok {
		return
	}
	if fieldID, ok := value["fieldId"].(string); ok {
		into[fieldID] = true
	}
	for _, field := range []string{"children", "initialItems"} {
		if children, ok := value[field].([]any); ok {
			for _, child := range children {
				collectFieldIDs(child, into)
			}
		}
	}
}
