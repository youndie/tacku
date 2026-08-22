// The server cannot ask the client what it can draw, so it does two things instead. This file holds
// both, because they are one decision: SPEC.md §2.2 forbids sending a form type outside "the profile
// the client is declared for", and nothing in the contract carries that profile — no header in
// §16.7, and §2.4 says the profile is not the client's runtime contract at all. The question is
// Q-26 in the journal; what follows is what was decided in its place.
package httpsrv_test

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/youndie/tacku/server/internal/httpsrv"
	"github.com/youndie/tacku/server/internal/spec"
)

// One tree for everybody.
//
// The rejected alternative was reading the client's abilities off its User-Agent. The version of an
// application and the version of its vocabulary are not the same thing, and they diverge exactly
// when it is most expensive — so a build that branches on it would be wrong in the one case the
// branch exists for. This test is that decision made executable: the answer must not move when the
// request claims to come from a particular client, however it claims it.
func TestTheAnswerDoesNotDependOnWhoIsAsking(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	claims := []map[string]string{
		{"User-Agent": "tacku-desktop/0.1.0 (Compose; kompot 0.15.0.22)"},
		{"User-Agent": "tacku-desktop/0.0.1"},
		{"X-Kompot-Profile": "2026-01-01"},
		{"X-Client-Version": "0.0.1", "User-Agent": "curl/8.7.1"},
	}

	// Every described endpoint rather than a list written here. The first version of this test named
	// four paths and stayed green while a fifth one was taught to branch on the User-Agent — a
	// check looking in the wrong place, which is the failure mode this project treats as the
	// dangerous one.
	compared := 0
	for path := range describedGets(t, r.url) {
		_, plain := r.get(t, path, token, "")
		if len(plain) == 0 {
			t.Fatalf("%s answered nothing, so this check has nothing to compare", path)
		}
		for _, claim := range claims {
			_, answered := r.getWith(t, path, token, claim)
			if string(answered) != string(plain) {
				t.Errorf("%s answered differently to a caller sending %v; the server has started "+
					"guessing at client abilities it has no way to know", path, claim)
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("nothing was compared, so this check proves nothing")
	}
	t.Logf("compared %d answers", compared)
}

// Nothing this server sends carries a wire type its own profile does not name.
//
// This is the only guarantee available once asking is off the table (§2.4: an implementation must
// confine itself to the profile it claims), and it is the one that matters most where the protocol
// does not degrade. The check is driven by the description rather than by a list written here, so an
// endpoint added later is covered by having been described — and it counts what it reached, because
// a walk that found no nodes of a hierarchy would pass in silence.
func TestNoResponseCarriesATypeOutsideOurOwnProfile(t *testing.T) {
	r := newResource(t)
	r.seed(t)
	token := r.reader(t)

	loaded, err := spec.Load(specDir(t))
	if err != nil {
		t.Fatal(err)
	}

	visited := map[string]int{}
	scanned := 0

	for path, operation := range describedGets(t, r.url) {
		response, body := r.get(t, path, token, "")
		if response.StatusCode != http.StatusOK {
			t.Errorf("%s answered %d: %s", path, response.StatusCode, body)
			continue
		}
		result, err := loaded.Scan(operation.reference, body)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		scanned++
		if testing.Verbose() {
			t.Logf("%s: %s", path, counts(result.Visited))
		}
		for hierarchy, count := range result.Visited {
			visited[hierarchy] += count
		}
		for _, found := range result.Undeclared {
			t.Errorf("%s %s", path, found)
		}
	}

	if scanned == 0 {
		t.Fatal("no endpoint was walked, so this check proves nothing")
	}

	// The counts are printed and not only asserted: a hierarchy dropping to zero is how this check
	// would quietly stop testing anything, and the number is the first thing to look at when it
	// does.
	t.Logf("walked %d endpoints; nodes reached: %s", scanned, counts(visited))

	// Every hierarchy that does not degrade has to be either reached or excused by name. The list
	// comes out of the schema files, so a kompot release adding a fifth one turns this check red
	// instead of leaving it looking at four.
	unsent := map[string]string{
		"FormCondition":    "this build sends no visibleIf, so no condition travels",
		"WizardTransition": "a request type: it travels from the client, never in an answer",
	}
	dangerous := loaded.NonDegrading()
	if len(dangerous) == 0 {
		t.Fatal("no non-degrading hierarchy was found in the schema files, so this check has no targets")
	}
	for _, hierarchy := range dangerous {
		if visited[hierarchy] > 0 {
			continue
		}
		if reason, excused := unsent[hierarchy]; excused {
			t.Logf("%s was not reached: %s", hierarchy, reason)
			continue
		}
		t.Errorf("the walk reached no %s at all, and it is a hierarchy with no fallback: either the "+
			"check stopped looking there, or this build has started sending something new", hierarchy)
	}

	// The degrading half is cheap to get wrong and cheap to check, and its counts are also what
	// tells a reader that the walk went through the trees at all.
	for _, hierarchy := range []string{"KompotComponent", "KompotAction"} {
		if visited[hierarchy] == 0 {
			t.Errorf("the walk reached no %s at all", hierarchy)
		}
	}
}

type describedOperation struct {
	reference string
}

// describedGets reads the OpenAPI description for the endpoints that can be asked without changing
// anything, together with the schema each one promises to answer with.
func describedGets(t *testing.T, resource string) map[string]describedOperation {
	t.Helper()

	var document struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]struct {
					Schema struct {
						Ref string `json:"$ref"`
					} `json:"schema"`
				} `json:"content"`
			} `json:"responses"`
			Parameters []struct {
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(httpsrv.OpenAPI(resource), &document); err != nil {
		t.Fatal(err)
	}

	gets := map[string]describedOperation{}
	for template, methods := range document.Paths {
		operation, ok := methods["get"]
		if !ok {
			continue
		}
		path := template
		for _, parameter := range operation.Parameters {
			if parameter.In == "path" {
				path = strings.ReplaceAll(path, "{"+parameter.Name+"}", "TAC-1")
			}
		}
		reference := operation.Responses["200"].Content["application/json"].Schema.Ref
		if reference == "" {
			t.Fatalf("%s promises no schema for its answer, so nothing can be checked against it", template)
		}
		gets[path] = describedOperation{reference: reference}
	}
	if len(gets) == 0 {
		t.Fatal("the description names no readable endpoint")
	}
	return gets
}

func counts(visited map[string]int) string {
	names := make([]string, 0, len(visited))
	for name := range visited {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+strconv.Itoa(visited[name]))
	}
	return strings.Join(parts, " ")
}
