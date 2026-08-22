package httpsrv

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/youndie/tacku/server/internal/store/sqlite"
)

// The list a step declares against the schema it builds.
//
// The list is not decoration: it decides which values a transition may overwrite, and a step that
// grew a field without saying so would keep whatever was entered into that field on an earlier
// visit — a value nobody can see, arriving in the task at the end. The failure is silent in both
// directions, which is why the two halves are compared rather than trusted.
func TestEveryStepDeclaresExactlyTheFieldsItBuilds(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "tacku.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	if _, err := store.CreateBoard(ctx, "Sprint 24"); err != nil {
		t.Fatal(err)
	}

	flow := newTaskFlow()
	if len(flow) < 2 {
		t.Fatalf("the flow has %d steps, so nothing multi-step is being checked", len(flow))
	}

	for _, step := range flow {
		built, err := step.build(ctx, store, nil)
		if err != nil {
			t.Fatalf("%s: %v", step.id, err)
		}

		encoded, err := json.Marshal(built.Schema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			FormID string `json:"formId"`
			Fields []struct {
				FieldID string `json:"fieldId"`
			} `json:"fields"`
		}
		if err := json.Unmarshal(encoded, &schema); err != nil {
			t.Fatal(err)
		}

		if schema.FormID != step.formID {
			t.Errorf("step %s is declared as form %q and builds %q", step.id, step.formID, schema.FormID)
		}
		if len(schema.Fields) == 0 {
			t.Errorf("step %s builds no field at all, so this comparison proves nothing", step.id)
		}

		shown := make([]string, 0, len(schema.Fields))
		for _, field := range schema.Fields {
			shown = append(shown, field.FieldID)
			if !slices.Contains(step.fields, field.FieldID) {
				t.Errorf("step %s shows %q, which it does not declare: a value entered there would "+
					"survive a visit that cleared it", step.id, field.FieldID)
			}
		}
		for _, declared := range step.fields {
			if !slices.Contains(shown, declared) {
				t.Errorf("step %s declares %q and does not show it: the transition would clear a "+
					"value the person was never asked for", step.id, declared)
			}
		}
	}
}
