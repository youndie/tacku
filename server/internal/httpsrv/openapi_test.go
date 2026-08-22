package httpsrv_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
			Kind       string         `json:"x-kompot-endpoint-kind"`
			Responses  map[string]any `json:"responses"`
			Security   *[]any         `json:"security"`
			Parameters []struct {
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(httpsrv.OpenAPI(r.url), &document); err != nil {
		t.Fatal(err)
	}

	kinds := map[string]bool{"screen": true, "form": true, "page": true, "submit": true,
		"patch": true, "data_source": true, "wizard_start": true, "wizard_resume": true,
		"graph": true, "updates_stream": true}

	for templatePath, methods := range document.Paths {
		for method, operation := range methods {
			// A templated path is asked with a value in it. Checking the template literally tests
			// whether the server serves a route named `{task}`, which it does not and should not.
			path := templatePath
			for _, parameter := range operation.Parameters {
				if parameter.In == "path" {
					path = strings.ReplaceAll(path, "{"+parameter.Name+"}", "TAC-1")
				}
			}
			if !kinds[operation.Kind] {
				t.Errorf("%s %s declares kind %q, which the protocol does not define", method, path, operation.Kind)
			}
			// `security: []` marks the two routes a person with no session must reach. The test
			// reads the declaration rather than assuming, which is the point of having one: an
			// endpoint documented as protected and answering 200 anonymously is the defect this
			// check exists for, in either direction.
			public := operation.Security != nil && len(*operation.Security) == 0

			if !public {
				if _, ok := operation.Responses["401"]; !ok {
					t.Errorf("%s %s is behind a token check and does not declare 401", method, path)
				}
			}

			// A submit is asked with the method it declares and without the idempotency key, so what
			// is checked is the refusal §16.5 requires rather than the operation itself, which would
			// create a task on every run.
			switch {
			case method == "post" && !public:
				refused := r.post(t, path, token, "", `{"formId":"x","fieldId":"","values":{}}`)
				if refused.StatusCode != 400 {
					t.Errorf("%s %s without an idempotency key answered %d, want 400",
						method, path, refused.StatusCode)
				}
			// A live channel is asked for and let go of rather than read to the end: it answers
			// frames until the reader leaves, so reading it like a document waits for an end that
			// never arrives. What this check wants from it is the same as from every other route —
			// that something is mounted there and answers 200 — and that is known from the headers.
			case method == "get" && operation.Kind == "updates_stream":
				if status := r.head(t, path, token); status != 200 {
					t.Errorf("the description promises %s %s, which answered %d", method, path, status)
				}
			case method == "get":
				response, _ := r.get(t, path, token, "")
				if response.StatusCode != 200 {
					t.Errorf("the description promises %s %s, which answered %d", method, path, response.StatusCode)
				}
			}

			if !public {
				anonymous := r.request(t, methodOf(method), path, "", "")
				if anonymous.StatusCode != 401 {
					t.Errorf("%s %s answered %d to an anonymous caller while declaring 401",
						method, path, anonymous.StatusCode)
				}
			} else if method == "get" {
				open, _ := r.get(t, path, "", "")
				if open.StatusCode != 200 {
					t.Errorf("%s %s is declared public and answered %d without a token", method, path, open.StatusCode)
				}
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
