package httpsrv_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Every form this server draws submits somewhere that answers.
//
// This is the counterpart of the check that every deeplink resolves, and it did not exist while the
// defect it would have caught was on screen: the comment box on a task declared one identifier, the
// route was mounted under another, and pressing "Post" sent a request into nothing. Nothing failed.
// A POST to an address with no handler and a button nobody pressed look identical from the outside,
// which is why this has to be a check rather than a habit.
//
// The rule it enforces is the one that replaced a table of exceptions in the client: a form is
// named after the address it submits to, so `/submit/<formId>` is the whole mapping.
func TestEveryFormSubmitsSomewhereThatAnswers(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	screens := screenPaths()

	forms := map[string]string{}
	for _, path := range screens {
		_, body := r.get(t, path, token, "")
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, formID := range submitTargets(tree) {
			forms[formID] = path
		}
	}

	if len(forms) == 0 {
		t.Fatal("no screen carried a submit at all, so this check had nothing to look at")
	}

	for formID, screen := range forms {
		address := "/submit/" + formID
		// A key, and deliberately so. The idempotency middleware wraps the whole mux and runs
		// before routing, so a POST without one is refused at 400 whether or not the address
		// exists — and this check would pass for every address in the world.
		response := r.post(t, address, token, "reaches-"+formID, `{"formId":"`+formID+`","fieldId":"","values":{}}`)
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()

		// A handler refusing an empty submission also answers 404 — "no such board" is not the same
		// event as "no such address", and a check that cannot tell them apart would call a working
		// endpoint broken. The router's own 404 is the plain-text one it writes itself; ours is
		// JSON, because everything this server refuses is refused in JSON.
		routed := response.Header.Get("Content-Type") == "application/json"
		if response.StatusCode == http.StatusNotFound && !routed {
			t.Errorf("%s draws a form submitting to %q and nothing is mounted there — the button does nothing, silently: %s",
				screen, address, strings.TrimSpace(string(body)))
		}
	}
}

// submitTargets collects the form identifier of every submit_form action in a tree.
func submitTargets(node any) []string {
	value, ok := node.(map[string]any)
	if !ok {
		return nil
	}

	found := []string{}
	if action, ok := value["action"].(map[string]any); ok {
		if action["type"] == "submit_form" {
			if formID, ok := action["formId"].(string); ok {
				found = append(found, formID)
			}
		}
	}
	for _, field := range []string{"children", "initialItems"} {
		if children, ok := value[field].([]any); ok {
			for _, child := range children {
				found = append(found, submitTargets(child)...)
			}
		}
	}
	for _, field := range []string{"emptyState", "screen"} {
		if child, ok := value[field].(map[string]any); ok {
			found = append(found, submitTargets(child)...)
		}
	}
	return found
}

// The filtered list names where its values go, and that address answers.
//
// The client-side half of this is FilterWireTest, which drives the published toolkit and shows that
// a value reaches the loader when `reloadUrl` is set. This is the other half: that our screen sets
// it, and sets it to somewhere mounted. Either half alone proves nothing useful — the toolkit
// working does not mean this server asked it to, and an address in a body does not mean anything
// answers there.
func TestTheFilterNamesAnAddressThatAnswers(t *testing.T) {
	r := newResource(t)
	r.fill(t, 3)
	token := r.reader(t)

	_, body := r.get(t, "/forms/my-tasks", token, "")
	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}

	address := reloadURLOf(tree)
	if address == "" {
		t.Fatal("the filtered list carries no reloadUrl, so choosing a filter changes a box on the screen and nothing else")
	}

	// With the field values on it, which is the shape the toolkit sends and the shape this handler
	// has always read.
	response, page := r.get(t, address+"?status=in_progress", token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %d", address, response.StatusCode)
	}
	if !json.Valid(page) {
		t.Errorf("%s answered something that is not JSON", address)
	}
}

func reloadURLOf(node any) string {
	value, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	if url, ok := value["reloadUrl"].(string); ok && url != "" {
		return url
	}
	for _, field := range []string{"children", "initialItems"} {
		if children, ok := value[field].([]any); ok {
			for _, child := range children {
				if found := reloadURLOf(child); found != "" {
					return found
				}
			}
		}
	}
	for _, field := range []string{"emptyState", "screen"} {
		if child, ok := value[field].(map[string]any); ok {
			if found := reloadURLOf(child); found != "" {
				return found
			}
		}
	}
	return ""
}
