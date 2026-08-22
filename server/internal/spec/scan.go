package spec

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Undeclared is one node of a response whose wire type this build's profile does not name.
//
// Degrades separates the two halves of the protocol, because the price of the same mistake is not
// the same in them. An unknown KompotComponent or KompotAction costs a placeholder (SPEC.md §2.1);
// an unknown FormFieldDefinition, ValidationRule, FieldValue or FormCondition costs the parse of
// the whole response (§2.2). Both are violations of §2.4 — a server must confine itself to the
// profile it claims — but only the second one empties a screen.
type Undeclared struct {
	Path      string
	Hierarchy string
	WireType  string
	Degrades  bool
}

func (u Undeclared) String() string {
	cost := "the whole response fails to parse"
	if u.Degrades {
		cost = "the node degrades to a placeholder"
	}
	return fmt.Sprintf("%s: %s %q is not in the profile (%s)", u.Path, u.Hierarchy, u.WireType, cost)
}

// ScanResult is what one walk of a response found.
//
// Visited counts the nodes of each hierarchy the walk actually reached, and it is there for the
// same reason the conformance gate counts targets: a check that found nothing to look at proves
// nothing, and without the count it is indistinguishable from a check that looked and was happy.
type ScanResult struct {
	Undeclared []Undeclared
	Visited    map[string]int
}

// NonDegrading names the hierarchies of this contract that have no runtime fallback — the ones
// where an unknown type costs the parse of the whole response rather than one node.
//
// Read out of the schema files rather than written down here, and that is the point: the list is
// four names today (SPEC.md §2.2) plus the two hierarchies closed outright (§2.3 and the wizard's
// transitions), and a kompot release that adds a fifth would otherwise arrive with nothing looking
// at it. A list typed into this file would still say four.
func (s *Spec) NonDegrading() []string {
	names := map[string]bool{}
	for file, document := range s.Schemas {
		if file == ProfileFileName {
			continue
		}
		var parsed struct {
			Defs map[string]node `json:"$defs"`
		}
		if err := json.Unmarshal(document, &parsed); err != nil {
			continue
		}
		for name, definition := range parsed.Defs {
			if definition.Kind != "hierarchy" {
				continue
			}
			if definition.Degrades == nil || !*definition.Degrades {
				names[name] = true
			}
		}
	}

	list := make([]string, 0, len(names))
	for name := range names {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

// Scan walks a response body guided by the schema files and reports every node whose wire type this
// build's profile does not declare.
//
// Why a walk rather than one more schema validation: the module schemas point at the *open* bases on
// purpose (§2.1), so a form envelope validated against `kompot-forms#/$defs/KompotFormResponse`
// reaches `FormFieldDefinition` as "an object with a type" and accepts a field type nobody
// declared. The closed lists live only in the profile, and the profile holds the six hierarchies
// with no envelope wrapped around them — so there is no single reference that closes a whole
// response. The walk supplies the missing half: it follows the schema down and applies the profile
// at every hierarchy it passes through.
//
// reference names the root, in the form produced by In: "kompot-forms.schema.json#/$defs/Name".
func (s *Spec) Scan(reference string, body []byte) (*ScanResult, error) {
	var instance any
	if err := json.Unmarshal(body, &instance); err != nil {
		return nil, fmt.Errorf("spec: the body is not JSON: %w", err)
	}

	sc := &scanner{spec: s, defs: map[string]map[string]json.RawMessage{},
		result: &ScanResult{Visited: map[string]int{}}}

	file, name, err := split("", reference)
	if err != nil {
		return nil, err
	}
	definition, err := sc.definition(file, name)
	if err != nil {
		return nil, err
	}
	if err := sc.walk(instance, file, name, definition, "$", 0); err != nil {
		return nil, err
	}

	sort.Slice(sc.result.Undeclared, func(i, j int) bool {
		return sc.result.Undeclared[i].Path < sc.result.Undeclared[j].Path
	})
	return sc.result, nil
}

type scanner struct {
	spec   *Spec
	defs   map[string]map[string]json.RawMessage
	result *ScanResult
}

// node is the subset of JSON Schema this walk needs. Everything else — types, patterns, required —
// is the validator's job and is checked there; duplicating it here would be a second, weaker
// implementation of a job already done.
type node struct {
	Kind          string                     `json:"x-kompot-kind"`
	Degrades      *bool                      `json:"x-kompot-degrades"`
	Discriminator *discriminator             `json:"discriminator"`
	Ref           string                     `json:"$ref"`
	Properties    map[string]json.RawMessage `json:"properties"`
	Items         json.RawMessage            `json:"items"`
	Additional    json.RawMessage            `json:"additionalProperties"`
	AnyOf         []json.RawMessage          `json:"anyOf"`
	OneOf         []json.RawMessage          `json:"oneOf"`
}

type discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping"`
}

// maxDepth bounds the walk. The instance is finite, but a chain of $refs is not obliged to consume
// any of it, and a schema that pointed at itself would otherwise hang the test suite rather than
// fail it.
const maxDepth = 128

func (sc *scanner) walk(instance any, file, name string, schema json.RawMessage, path string, depth int) error {
	if instance == nil || len(schema) == 0 {
		return nil
	}
	if depth > maxDepth {
		return fmt.Errorf("spec: the walk is %d levels deep at %s; the schema refers to itself", depth, path)
	}

	var n node
	if err := json.Unmarshal(schema, &n); err != nil {
		return fmt.Errorf("spec: %s#/$defs/%s: %w", file, name, err)
	}

	if n.Ref != "" {
		nextFile, nextName, err := split(file, n.Ref)
		if err != nil {
			return err
		}
		definition, err := sc.definition(nextFile, nextName)
		if err != nil {
			return err
		}
		return sc.walk(instance, nextFile, nextName, definition, path, depth+1)
	}

	if n.Kind == "hierarchy" {
		return sc.hierarchy(instance, file, name, n, path, depth)
	}

	switch value := instance.(type) {
	case map[string]any:
		for key, held := range value {
			if property, ok := n.Properties[key]; ok {
				if err := sc.walk(held, file, name, property, path+"."+key, depth+1); err != nil {
					return err
				}
				continue
			}
			// additionalProperties is a schema for the values of a map — which is how every
			// FieldValue payload travels — and `true` for an object that merely tolerates extra
			// keys (§3). Only the first form is something to walk into.
			if isSchema(n.Additional) {
				if err := sc.walk(held, file, name, n.Additional, path+"."+key, depth+1); err != nil {
					return err
				}
			}
		}
	case []any:
		for index, held := range value {
			if err := sc.walk(held, file, name, n.Items, fmt.Sprintf("%s[%d]", path, index), depth+1); err != nil {
				return err
			}
		}
	}

	// A nullable hierarchy is written as anyOf[$ref, null] rather than as a plain $ref, so the
	// branches have to be followed or `visibleIf` would never be looked at.
	for _, branch := range append(append([]json.RawMessage{}, n.AnyOf...), n.OneOf...) {
		if err := sc.walk(instance, file, name, branch, path, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (sc *scanner) hierarchy(instance any, file, name string, n node, path string, depth int) error {
	object, ok := instance.(map[string]any)
	if !ok {
		// A hierarchy that arrived as something other than an object is a shape error, and shape is
		// the validator's half of the job. Reporting it here as well would give one defect two
		// voices.
		return nil
	}

	property := "type"
	if n.Discriminator != nil && n.Discriminator.PropertyName != "" {
		property = n.Discriminator.PropertyName
	}
	wireType, _ := object[property].(string)

	sc.result.Visited[name]++

	// Which mapping closes this hierarchy depends on who owns the closing. The open ones (§2.1,
	// §2.2) are closed by the build, so the list is in the profile; KompotModifierNode is closed by
	// the protocol itself (§2.3) and carries its list in the module schema.
	mapping := sc.profileMapping(name)
	if mapping == nil && n.Discriminator != nil {
		mapping = n.Discriminator.Mapping
	}
	if len(mapping) == 0 {
		return fmt.Errorf("spec: hierarchy %s is closed neither by the profile nor by %s, so nothing can be checked against it", name, file)
	}

	target, declared := mapping[wireType]
	if !declared {
		sc.result.Undeclared = append(sc.result.Undeclared,
			Undeclared{Path: path, Hierarchy: name, WireType: wireType, Degrades: sc.degrades(name, n)})
		return nil
	}

	nextFile, nextName, err := split(file, target)
	if err != nil {
		return err
	}
	definition, err := sc.definition(nextFile, nextName)
	if err != nil {
		return err
	}
	return sc.walk(instance, nextFile, nextName, definition, path, depth+1)
}

// degrades answers whether an unknown type of this hierarchy costs a node or the whole response.
//
// The flag is declared on the base definition in the module that owns the hierarchy, and a walk can
// reach the hierarchy through the profile instead — where the closed list lives but the flag does
// not. Taking the answer from whichever definition carries it keeps the two halves apart no matter
// which door the walk came in through.
func (sc *scanner) degrades(hierarchy string, arrived node) bool {
	if arrived.Degrades != nil {
		return *arrived.Degrades
	}
	for file := range sc.spec.Schemas {
		if file == ProfileFileName {
			continue
		}
		definition, err := sc.definition(file, hierarchy)
		if err != nil {
			continue
		}
		var base node
		if err := json.Unmarshal(definition, &base); err != nil {
			continue
		}
		if base.Degrades != nil {
			return *base.Degrades
		}
	}
	return false
}

func (sc *scanner) profileMapping(hierarchy string) map[string]string {
	var document struct {
		Defs map[string]struct {
			Discriminator discriminator `json:"discriminator"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(sc.spec.Profile, &document); err != nil {
		return nil
	}
	def, ok := document.Defs[hierarchy]
	if !ok {
		return nil
	}
	return def.Discriminator.Mapping
}

func (sc *scanner) definition(file, name string) (json.RawMessage, error) {
	defs, ok := sc.defs[file]
	if !ok {
		document, held := sc.spec.Schemas[file]
		if !held {
			return nil, fmt.Errorf("spec: %s is referenced but the profile does not name it", file)
		}
		var parsed struct {
			Defs map[string]json.RawMessage `json:"$defs"`
		}
		if err := json.Unmarshal(document, &parsed); err != nil {
			return nil, fmt.Errorf("spec: %s: %w", file, err)
		}
		defs = parsed.Defs
		sc.defs[file] = defs
	}

	definition, ok := defs[name]
	if !ok {
		return nil, fmt.Errorf("spec: %s declares no %s", file, name)
	}
	return definition, nil
}

// split resolves a reference against the file it was found in. Both forms the schemas use are
// accepted: "#/$defs/Name" inside one file and "other.schema.json#/$defs/Name" across files.
func split(from, reference string) (file, name string, err error) {
	const defs = "#/$defs/"
	index := strings.Index(reference, defs)
	if index < 0 {
		return "", "", fmt.Errorf("spec: %q does not name a definition", reference)
	}
	file = reference[:index]
	if file == "" {
		file = from
	}
	if file == "" {
		return "", "", fmt.Errorf("spec: %q is relative and there is no file to resolve it against", reference)
	}
	return file, reference[index+len(defs):], nil
}

func isSchema(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{")
}
