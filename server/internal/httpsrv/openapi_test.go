package httpsrv_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/youndie/tacku/server/internal/httpsrv"
)

// The committed description against the generator.
//
// The same guard the Kotlin half puts on the schema files, and for the same reason: the conformance
// harness reads a file, and a file can fall behind the code that produced it. Regenerate with
//
//	go run ./cmd/tacku openapi > ../spec/kompot.openapi.json
func TestTheCommittedDescriptionMatchesTheGenerator(t *testing.T) {
	path := filepath.Join("..", "..", "..", "spec", "kompot.openapi.json")
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	generated := httpsrv.OpenAPI("http://localhost:8080")
	if string(committed) != string(generated) {
		t.Errorf("%s has drifted from the generator; regenerate it", path)
	}
}

// Every route the description names must be one the server actually mounts, and every kind it
// declares must be one the protocol defines. A description that promises a path nobody serves sends
// a conformance run looking for something that was never there.
func TestTheDescriptionOnlyPromisesRoutesThatExist(t *testing.T) {
	r := newResource(t)
	token := r.reader(t)
	r.seed(t)

	var document struct {
		Paths map[string]map[string]struct {
			Kind      string          `json:"x-kompot-endpoint-kind"`
			Responses map[string]any  `json:"responses"`
			Operation json.RawMessage `json:"-"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(httpsrv.OpenAPI(r.url), &document); err != nil {
		t.Fatal(err)
	}

	kinds := map[string]bool{"screen": true, "form": true, "page": true, "submit": true,
		"patch": true, "data_source": true, "wizard_start": true, "wizard_resume": true,
		"graph": true, "updates_stream": true}

	for path, methods := range document.Paths {
		for method, operation := range methods {
			if !kinds[operation.Kind] {
				t.Errorf("%s %s declares kind %q, which the protocol does not define", method, path, operation.Kind)
			}
			if _, ok := operation.Responses["401"]; !ok {
				t.Errorf("%s %s is behind the bearer check and does not declare 401", method, path)
			}

			// A submit is asked with the method it declares and without the idempotency key, so
			// what is checked is the refusal §16.5 requires rather than the operation itself —
			// which would create a task on every run of this test.
			if method == "post" {
				refused := r.post(t, path, token, "", `{"formId":"x","fieldId":"","values":{}}`)
				if refused.StatusCode != 400 {
					t.Errorf("%s %s without an idempotency key answered %d, want 400",
						method, path, refused.StatusCode)
				}
			} else {
				response, _ := r.get(t, path, token, "")
				if response.StatusCode != 200 {
					t.Errorf("the description promises %s %s, which answered %d", method, path, response.StatusCode)
				}
			}

			anonymous := r.request(t, methodOf(method), path, "", "")
			if anonymous.StatusCode != 401 {
				t.Errorf("%s %s answered %d to an anonymous caller while declaring 401", method, path, anonymous.StatusCode)
			}
		}
	}
}

func methodOf(declared string) string {
	if declared == "post" {
		return http.MethodPost
	}
	return http.MethodGet
}
