package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator checks a response against the schema of this build.
//
// The mitigation for the risk this project carries by construction: the form hierarchies do not
// degrade, so a wire type the client does not know fails the parse of a whole screen rather than one
// node — and Go has no compiler to stop a wrong string. Every response is therefore checked against
// the profile before it can reach anybody.
type Validator struct {
	compiler *jsonschema.Compiler
	base     string
}

// NewValidator registers every schema file under the identifier its $id declares, so that the
// relative $refs between them resolve exactly as they do for any other consumer.
func NewValidator(s *Spec) (*Validator, error) {
	// Go's own regexp engine, again.
	//
	// It briefly was not: the deeplink pattern used a negative lookahead, which ECMA-262 has and RE2
	// has not, so no Go implementation using the standard library could compile the schema — let
	// alone validate a response. Reported, and kompot answered by splitting the rule into a
	// `pattern` and a neighbouring `not`, which says the same thing in schema keywords and compiles
	// everywhere. The dependency that stood in for the missing engine is gone with it.
	compiler := jsonschema.NewCompiler()

	var base string
	for name, document := range s.Schemas {
		var parsed struct {
			ID string `json:"$id"`
		}
		if err := json.Unmarshal(document, &parsed); err != nil {
			return nil, fmt.Errorf("spec: %s: %w", name, err)
		}
		if parsed.ID == "" {
			return nil, fmt.Errorf("spec: %s declares no $id, so nothing can $ref it", name)
		}
		if base == "" {
			base = strings.TrimSuffix(parsed.ID, name)
		}

		resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
		if err != nil {
			return nil, fmt.Errorf("spec: %s: %w", name, err)
		}
		if err := compiler.AddResource(parsed.ID, resource); err != nil {
			return nil, fmt.Errorf("spec: %s: %w", name, err)
		}
	}

	return &Validator{compiler: compiler, base: base}, nil
}

// Profile names a definition of the build's profile — the closed list of types.
func Profile(definition string) string { return ProfileFileName + "#/$defs/" + definition }

// In names a definition of one module's schema, for the envelopes the profile does not hold: it
// carries the hierarchies and not the shapes wrapped around them.
func In(module, definition string) string { return module + ".schema.json#/$defs/" + definition }

// Validate checks a document against a reference built by Profile or In.
func (v *Validator) Validate(reference string, body []byte) error {
	schema, err := v.compiler.Compile(v.base + reference)
	if err != nil {
		return fmt.Errorf("spec: compiling %s: %w", reference, err)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("spec: the body is not JSON: %w", err)
	}

	if err := schema.Validate(instance); err != nil {
		return fmt.Errorf("spec: %s does not satisfy %s: %w", shorten(body), reference, err)
	}
	return nil
}

func shorten(body []byte) string {
	const limit = 120
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "…"
}
