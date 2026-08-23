//go:build !debugdoor

package httpsrv_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/youndie/tacku/server/internal/httpsrv"
)

// The addresses the page keeps are answered with the page, and nothing else is.
//
// Both halves matter and the second is the one at risk. A server that answers every unknown path
// with the page turns a typo into a blank screen: the person sees the product start and then say
// nothing, instead of a 404 that names what is wrong. The old rule refused everything because there
// were no client addresses at all; now there are exactly the ones the graph names.
func TestThePageAnswersItsOwnAddresses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>tacku</title>"), 0o600); err != nil {
		t.Fatal(err)
	}

	resource := newResourceWith(t, func(config *httpsrv.Config) { config.PageDir = dir })

	kept := []string{"/board", "/catch-up", "/my-tasks", "/new-task", "/task/TAC-2", "/edit-task/TAC-2"}
	for _, path := range kept {
		if code := ask(t, resource.url+path, "").StatusCode; code != http.StatusOK {
			t.Errorf("%s answered %d: a person reloading that screen gets no page", path, code)
		}
	}

	// The last two are the ones that cost a blank window: a page served at a screen address asks for
	// its own script relatively, and answering that with the page hands the browser HTML where it
	// expects JavaScript.
	strangers := []string{"/borad", "/task", "/tasks/TAC-2", "/nothing/at/all", "/task/tacku.js", "/edit-task/style.css"}
	for _, path := range strangers {
		if code := ask(t, resource.url+path, "").StatusCode; code != http.StatusNotFound {
			t.Errorf("%s answered %d: a typo was given the page instead of an answer", path, code)
		}
	}
}
