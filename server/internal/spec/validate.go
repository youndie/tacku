package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
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
	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(ecmaRegexp)

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

// ecmaRegexp replaces Go's regexp engine with one that speaks ECMA-262.
//
// Not a preference. JSON Schema specifies `pattern` in the ECMA-262 dialect, which has lookaround;
// Go's standard regexp is RE2, which has none by design, and refuses to compile the expression at
// all. The protocol uses one: the deeplink pattern is
//
//	^(?!https?:)[a-z][a-z0-9+.-]*://[^\s]*$
//
// and the negative lookahead is the whole point of it — it is what stops a server sending a client
// to a web address through an ordinary navigate.
//
// So a schema that is valid, and a rule that matters, cannot be checked by any Go implementation
// using the standard library. A Kotlin one never meets this, its regexes being Java's. Reported
// upstream; the escape hatch here is documented by the validator itself.
func ecmaRegexp(pattern string) (jsonschema.Regexp, error) {
	compiled, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	return (*ecmaPattern)(compiled), nil
}

type ecmaPattern regexp2.Regexp

func (p *ecmaPattern) MatchString(value string) bool {
	matched, err := (*regexp2.Regexp)(p).MatchString(value)
	return err == nil && matched
}

func (p *ecmaPattern) String() string { return (*regexp2.Regexp)(p).String() }
