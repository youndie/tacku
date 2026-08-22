package httpsrv_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
)

// boxes walks a tree and returns every node of the given wire type, by fieldId.
func boxes(node any, wireType string, into map[string]map[string]any) {
	value, ok := node.(map[string]any)
	if !ok {
		return
	}
	if value["type"] == wireType {
		if fieldID, ok := value["fieldId"].(string); ok {
			into[fieldID] = value
		}
	}
	for _, field := range []string{"children", "initialItems"} {
		if children, ok := value[field].([]any); ok {
			for _, child := range children {
				boxes(child, wireType, into)
			}
		}
	}
}

// The two texts a person writes in a tracker are boxes they can see what they wrote in — and the
// schema half of both is an ordinary `text_field`.
//
// That asymmetry is the design of the extension rather than an accident of it. The component
// hierarchy degrades (§2.1), so a client that never heard of `multiline_input` loses a box; the
// field hierarchy does not (§2.2), and a type of our own there would have cost the whole response.
// Which is why this check looks at both halves: the same field would pass a check that only looked
// at the tree even if the definition had quietly become a type of ours.
func TestBothTextsAPersonWritesAreMultilineOverAnOrdinaryTextField(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	cases := []struct {
		path    string
		fieldID string
	}{
		{"/forms/new-task", "description"},
		{"/forms/task/TAC-1", "comment"},
	}

	for _, c := range cases {
		_, body := r.get(t, c.path, token, "")

		var form struct {
			Schema struct {
				Fields []map[string]any `json:"fields"`
			} `json:"schema"`
			Screen any `json:"screen"`
		}
		if err := json.Unmarshal(body, &form); err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}

		found := map[string]map[string]any{}
		boxes(form.Screen, "multiline_input", found)
		box, ok := found[c.fieldID]
		if !ok {
			t.Errorf("%s draws no multiline box for %q; it has %d of them", c.path, c.fieldID, len(found))
			continue
		}
		// Written out rather than left to the client's default: two defaults for one number is how
		// the wire and the screen come to disagree in silence.
		if lines, ok := box["minLines"].(float64); !ok || lines < 2 {
			t.Errorf("%s: %q asks for %v lines, which is not a box for prose", c.path, c.fieldID, box["minLines"])
		}

		declared := ""
		for _, field := range form.Schema.Fields {
			if field["fieldId"] == c.fieldID {
				declared, _ = field["type"].(string)
			}
		}
		if declared != "text_field" {
			t.Errorf("%s declares %q as %q; the definition must stay the toolkit's own type, or an"+
				" older client loses the whole form rather than one box", c.path, c.fieldID, declared)
		}
	}
}

// The product half: a description written by a person reaches the task.
//
// It could not, until now. The store has held a body since the first migration and agents fill it
// through MCP, but the human surface had no component that shows more than one line, so the form
// simply had no such field — the tracker was half a tracker for people and whole for agents.
func TestADescriptionWrittenByAPersonReachesTheTask(t *testing.T) {
	r := newResource(t)
	r.seedBoard(t)
	token := r.reader(t)

	const description = "Rotate the keys, then revoke the old ones.\n\nCoordinate with support first."
	encoded, err := json.Marshal(description)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"formId":"new-task","fieldId":"","values":{` +
		`"title":{"type":"text_value","text":"Rotate the SSO signing keys"},` +
		`"description":{"type":"text_value","text":` + string(encoded) + `},` +
		`"board":{"type":"entity_value","id":"Sprint 24","title":"Sprint 24"},` +
		`"status":{"type":"entity_value","id":"todo","title":"todo"}}}`

	response := r.post(t, "/submit/new-task", token, "description-1", body)
	answer, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the submit answered %d: %s", response.StatusCode, answer)
	}

	var action struct {
		Deeplink string `json:"deeplink"`
	}
	if err := json.Unmarshal(answer, &action); err != nil {
		t.Fatal(err)
	}
	id := domain.TaskID(action.Deeplink[strings.LastIndex(action.Deeplink, "/")+1:])

	task, err := r.store.Task(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Body != description {
		t.Errorf("the task holds %q, not what was written", task.Body)
	}

	// And it is on the screen the reader opens, in the same shape it was written: the newlines are
	// what the whole extension is about, and a screen that flattened them would make the box a lie.
	_, screen := r.get(t, "/forms/task/"+string(id), token, "")
	if !strings.Contains(string(screen), "Coordinate with support first.") {
		t.Errorf("the task screen does not carry the description that was written")
	}
}
