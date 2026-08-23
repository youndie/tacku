package httpsrv_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A choice of one is not offered as a choice — and a task created that way still lands on the board.
//
// The mockup draws the board as context on the create screen rather than as a field, and with a
// single board the server knows the answer. What makes this worth a test is the second half: the
// form no longer sends anything under that name, so if the server did not supply it, every task
// created here would be created onto no board at all — and the screen that lists them would simply
// be empty, which reads as "nothing was created" rather than as "created wrongly".
func TestOneBoardIsShownRatherThanOffered(t *testing.T) {
	resource := newResource(t)
	resource.fill(t, 1)
	token := resource.reader(t)

	t.Run("the form shows it", func(t *testing.T) {
		body := resource.body(t, token, "/forms/new-task")

		if strings.Contains(body, `"type":"select_input"`) && strings.Contains(body, `"fieldId":"board"`) {
			t.Error("a board was offered as a choice while there was only one to choose")
		}
		if !strings.Contains(body, `"read_only_field"`) || !strings.Contains(body, "Sprint 24") {
			t.Errorf("the board is neither offered nor shown, so nobody can tell where the task will go: %s", body)
		}
	})

	t.Run("a task created without one lands on it anyway", func(t *testing.T) {
		created := resource.submit(t, token, "/submit/new-task", "new-task", map[string]any{
			"title": map[string]any{"type": "text_value", "text": "Written with no board named"},
		})
		if created == "" {
			t.Fatal("the submission was refused")
		}

		// Asked of the board rather than of the whole store: a task created onto no board would not
		// be listed here at all, which is the failure being guarded against — it exists, and no
		// screen will ever show it.
		boards, err := resource.store.Boards(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		tasks, err := resource.store.Tasks(context.Background(), boards[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			if task.Title == "Written with no board named" {
				return
			}
		}
		t.Fatal("the task is not on the only board there is")
	})
}

// body fetches a screen and hands back its text, unparsed: what is asserted above is the shape of
// the tree, and a parser here would be a second opinion about it.
func (r *resource) body(t *testing.T, token, path string) string {
	t.Helper()

	answer := ask(t, r.url+path, token)
	if answer.StatusCode != 200 {
		t.Fatalf("%s answered %d", path, answer.StatusCode)
	}
	return string(r.bodyOf(t, answer))
}

// submit posts one form and returns the raw answer, or "" when it was refused.
func (r *resource) submit(t *testing.T, token, path, formID string, values map[string]any) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"formId": formID, "fieldId": "", "values": values})
	if err != nil {
		t.Fatal(err)
	}
	answer := r.post(t, path, token, "one-board-"+formID, string(payload))
	if answer.StatusCode != 200 {
		t.Logf("%s answered %d: %s", path, answer.StatusCode, r.bodyOf(t, answer))
		return ""
	}
	return string(r.bodyOf(t, answer))
}
