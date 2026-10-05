package httpsrv_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/spec"
)

// The two buttons whose answer is "the screen you pressed me on, again".
//
// Both answered a navigate to their own screen's deeplink until kompot 0.38 gave the protocol a word
// for it (§16.4). The navigate was right only by arrangement: it had to name the screen the press came
// from, and nothing in a request says that (Q-32) — so a move button placed on a second screen would
// have taken whoever pressed it to the board. `refresh` names nothing, which is the point, and this
// test holds both answers to it: a deeplink coming back here is the arrangement coming back.
//
// Each answer is also walked against this build's profile: a word the profile does not declare would
// reach a client that cannot run it, and the client would do nothing, silently (§2.1).
func TestAButtonThatChangesItsOwnScreenAnswersRefresh(t *testing.T) {
	r := newResource(t)
	token := r.reader(t)
	board := r.board(t)
	task, err := r.store.CreateTask(t.Context(),
		domain.Task{Board: board, Title: "Moved from the board"}, domain.Human("anna"))
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := spec.Load(specDir(t))
	if err != nil {
		t.Fatal(err)
	}

	presses := map[string]string{
		"/submit/move": `{"formId":"board","fieldId":"","values":{` +
			`"task":{"type":"text_value","text":"` + string(task.ID) + `"},` +
			`"status":{"type":"text_value","text":"in_progress"}}}`,
		"/submit/seen": `{"formId":"","fieldId":"","values":{}}`,
	}

	answered := 0
	for path, body := range presses {
		response := r.post(t, path, token, "refresh-"+path, body)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s answered %d", path, response.StatusCode)
		}
		raw := r.bodyOf(t, response)

		var answer map[string]any
		if err := json.Unmarshal(raw, &answer); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if answer["type"] != "refresh" {
			t.Errorf("%s answered %v; the screen the press came from is shown again with refresh (§16.4)",
				path, answer)
		}
		if _, named := answer["deeplink"]; named {
			t.Errorf("%s names a screen in its answer, which only the client knows", path)
		}

		result, err := loaded.Scan("kompot-core.schema.json#/$defs/KompotAction", raw)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, found := range result.Undeclared {
			t.Errorf("%s %s", path, found)
		}
		answered++
	}
	if answered != len(presses) {
		t.Fatalf("checked %d answers of %d", answered, len(presses))
	}
}
