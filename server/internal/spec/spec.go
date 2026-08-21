// Package spec loads the generated KOMPOT spec of this build.
//
// The files are produced by the Kotlin half of the repository (client/spec-gen) and committed,
// because this side cannot run the generator: the profile is derived from the SerialDescriptors of
// the wire types, and those live in Kotlin. What crosses the line is finished JSON, which is the
// same thing a foreign implementer would be handed.
package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ProfileFileName is fixed by the protocol, not by this project.
const ProfileFileName = "kompot.profile.schema.json"

// DirEnv overrides where the spec is read from.
const DirEnv = "TACKU_SPEC_DIR"

// Spec is one build's contract: the profile plus the schema of every module the profile names.
type Spec struct {
	Profile json.RawMessage
	Schemas map[string]json.RawMessage
	Modules []string
}

// Load reads the profile first and then exactly the module files it names.
//
// Driving the file list from the profile rather than from a directory listing is deliberate and
// copies what the conformance kit does: a stray file in the directory is then not part of the
// contract, and a module named by the profile with no file behind it is an error rather than a
// silent gap.
func Load(dir string) (*Spec, error) {
	if dir == "" {
		dir = os.Getenv(DirEnv)
	}
	if dir == "" {
		return nil, fmt.Errorf("spec: no directory given and %s is unset", DirEnv)
	}

	profile, err := readJSON(filepath.Join(dir, ProfileFileName))
	if err != nil {
		return nil, err
	}

	var head struct {
		Modules []string `json:"x-kompot-modules"`
	}
	if err := json.Unmarshal(profile, &head); err != nil {
		return nil, fmt.Errorf("spec: profile is not readable: %w", err)
	}
	if len(head.Modules) == 0 {
		return nil, fmt.Errorf("spec: profile names no modules")
	}

	schemas := make(map[string]json.RawMessage, len(head.Modules)+1)
	for _, module := range head.Modules {
		name := FileNameFor(module)
		document, err := readJSON(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		schemas[name] = document
	}
	schemas[ProfileFileName] = profile

	return &Spec{Profile: profile, Schemas: schemas, Modules: head.Modules}, nil
}

// FileNameFor maps a module name to its schema file.
func FileNameFor(module string) string { return module + ".schema.json" }

// Types returns the closed list of wire types of one hierarchy, read from the profile's
// discriminator mapping. The hierarchy is named as in the schema: KompotComponent, KompotAction,
// FormFieldDefinition, ValidationRule, FieldValue, FormCondition.
//
// An unknown hierarchy returns nil rather than an empty set, so that a caller can tell "nothing is
// registered" from "this hierarchy does not exist" — the difference between a real answer and a
// typo in the name.
func (s *Spec) Types(hierarchy string) []string {
	var document struct {
		Defs map[string]struct {
			Discriminator struct {
				Mapping map[string]string `json:"mapping"`
			} `json:"discriminator"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(s.Profile, &document); err != nil {
		return nil
	}

	def, ok := document.Defs[hierarchy]
	if !ok {
		return nil
	}

	types := make([]string, 0, len(def.Discriminator.Mapping))
	for wireType := range def.Discriminator.Mapping {
		types = append(types, wireType)
	}
	sort.Strings(types)
	return types
}

// Declares reports whether this build may put the given wire type of the hierarchy on the wire.
//
// SPEC.md §2.4: an implementation must confine itself to the profile it claims. For the form
// hierarchies that rule has teeth — an unrecognised type there breaks the parse of the whole
// response rather than degrading one node (§2.2).
func (s *Spec) Declares(hierarchy, wireType string) bool {
	for _, declared := range s.Types(hierarchy) {
		if declared == wireType {
			return true
		}
	}
	return false
}

func readJSON(path string) (json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("spec: %s is not valid JSON", path)
	}
	return json.RawMessage(raw), nil
}
