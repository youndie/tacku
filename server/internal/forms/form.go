// Package forms builds the form envelope: the data contract and the tree, together.
//
// The two halves must agree — every input must name a field the schema declares (§9.2) — and here
// they agree structurally rather than by inspection: one call registers the definition and returns
// the component, so a component referring to an undeclared field cannot be written.
//
// This is the most dangerous corner of the protocol and the care is proportionate. The form
// hierarchies do not degrade (§2.2): a type the client does not know fails the parse of the whole
// response rather than one node, so the cost of a mistake here is an empty screen.
package forms

import (
	"github.com/youndie/tacku/server/internal/render"
)

// Response is the envelope of a `form` endpoint.
type Response struct {
	Schema Schema           `json:"schema"`
	Screen render.Component `json:"screen"`
}

// Schema is the data contract half.
type Schema struct {
	FormID string  `json:"formId"`
	Fields []Field `json:"fields"`
}

// Field is one declaration. The concrete types come from form-standard, and their wire names are
// the closed list of the profile — nothing here may invent one.
type Field any

// Rule is a client-side pre-check. Business validation stays on the server and may still refuse
// (§9.5): the state can change between rendering a form and submitting it.
type Rule any

type textField struct {
	Type         string `json:"type"`
	FieldID      string `json:"fieldId"`
	Rules        []Rule `json:"rules"`
	KeyboardType string `json:"keyboardType,omitempty"`
	Mask         string `json:"mask,omitempty"`
}

type selectionField struct {
	Type    string `json:"type"`
	FieldID string `json:"fieldId"`
	Rules   []Rule `json:"rules,omitempty"`
}

type checkboxField struct {
	Type    string `json:"type"`
	FieldID string `json:"fieldId"`
	Rules   []Rule `json:"rules,omitempty"`
}

type requiredRule struct {
	Type         string `json:"type"`
	ErrorMessage string `json:"errorMessage"`
}

type regexRule struct {
	Type         string `json:"type"`
	Pattern      string `json:"pattern"`
	ErrorMessage string `json:"errorMessage"`
}

// Required must come first in a rule list.
//
// Order is significant and the reason is asymmetric: `regex` lets an empty value through, so that an
// optional field can be left blank. Put after it, `required` would still work; put before, it is
// what produces the message a person can act on.
//
// The message is finished text, not a translation key (§14).
func Required(message string) Rule { return requiredRule{Type: "required", ErrorMessage: message} }

// Regex checks a shape. It deliberately passes an empty value; pair it with Required when the field
// is mandatory.
func Regex(pattern, message string) Rule {
	return regexRule{Type: "regex", Pattern: pattern, ErrorMessage: message}
}

// Builder assembles both halves at once.
type Builder struct {
	formID string
	fields []Field
	seen   map[string]bool
}

func New(formID string) *Builder {
	return &Builder{formID: formID, seen: map[string]bool{}}
}

// TextInput declares a text field and returns the component bound to it.
//
// Returning the component from the call that registers the definition is the whole point: there is
// no way to place an input without declaring its field, and no way to declare a field this form
// does not show.
func (b *Builder) TextInput(fieldID, label, placeholder string, rules []Rule, options ...TextOption) render.Component {
	settings := textSettings{}
	for _, option := range options {
		option(&settings)
	}

	b.declare(fieldID, textField{
		Type: "text_field", FieldID: fieldID, Rules: nonNilRules(rules),
		KeyboardType: settings.keyboard, Mask: settings.mask,
	})

	return render.TextInput(componentID(fieldID), fieldID, label, placeholder, settings.mask, settings.secret)
}

type textSettings struct {
	keyboard string
	mask     string
	secret   bool
}

type TextOption func(*textSettings)

func Keyboard(kind string) TextOption { return func(s *textSettings) { s.keyboard = kind } }
func Mask(mask string) TextOption     { return func(s *textSettings) { s.mask = mask } }
func Secret() TextOption              { return func(s *textSettings) { s.secret = true } }

// Select declares a selection field and returns the input that shows it.
//
// Options belong to the component rather than to the schema (§9.8): the same field can be drawn as
// a dropdown or as a radio group, and the contract does not care which.
func (b *Builder) Select(fieldID, label, placeholder string, options []render.SelectOption, rules []Rule) render.Component {
	b.declare(fieldID, selectionField{Type: "selection_field", FieldID: fieldID, Rules: rules})
	return render.SelectInput(componentID(fieldID), fieldID, label, placeholder, options)
}

func (b *Builder) Checkbox(fieldID, label string) render.Component {
	b.declare(fieldID, checkboxField{Type: "checkbox_field", FieldID: fieldID})
	return render.CheckboxInput(componentID(fieldID), fieldID, label)
}

// declare panics on a duplicate identifier.
//
// A panic rather than an error because there is no runtime recovery: a form with two fields of one
// name is a programming mistake, it is caught by the first test that renders the form, and a value
// silently overwriting another would be found much later and by a person.
func (b *Builder) declare(fieldID string, field Field) {
	if b.seen[fieldID] {
		panic("forms: field " + fieldID + " is declared twice in " + b.formID)
	}
	b.seen[fieldID] = true
	b.fields = append(b.fields, field)
}

// Build returns the envelope. The tree is passed in rather than assembled here so that layout stays
// where layout belongs.
func (b *Builder) Build(screen render.Component) Response {
	return Response{Schema: Schema{FormID: b.formID, Fields: b.fields}, Screen: screen}
}

// FormID is what a submit action names.
func (b *Builder) FormID() string { return b.formID }

func componentID(fieldID string) string { return "field-" + fieldID }

func nonNilRules(rules []Rule) []Rule {
	if rules == nil {
		return []Rule{}
	}
	return rules
}

// dateField is the schema half of the deployment's own field type.
//
// Its wire name is declared in the profile as an extension (§2.4) rather than in a module of the
// toolkit, which is what makes it legal to send at all: a validator on any stack accepts a declared
// name and refuses an undeclared one. The mechanism arrived in kompot 0.17; before it, a type of
// our own was either invisible to every check or cost writing a Kotlin module of the protocol.
type dateField struct {
	Type    string `json:"type"`
	FieldID string `json:"fieldId"`
	Rules   []Rule `json:"rules"`
	Value   string `json:"value,omitempty"`
	Min     string `json:"min,omitempty"`
	Max     string `json:"max,omitempty"`
}

func (dateField) isField() {}

// DateInput declares a date and returns the node that draws it.
//
// The value on the wire stays a `text_value` holding an ISO date, so nothing downstream changes:
// the handler reads it exactly as it read the masked text box. What changes is what a person does —
// picking a day they can name instead of composing an ISO string in their head, which is the
// scenario the design named and the reason this type exists.
func (b *Builder) DateInput(fieldID, label, value, min, max, hint string, rules []Rule) render.Component {
	b.declare(fieldID, dateField{
		Type: "date_field", FieldID: fieldID, Rules: nonNilRules(rules),
		Value: value, Min: min, Max: max,
	})
	return render.DateInput(componentID(fieldID), fieldID, label, hint)
}

// MultilineInput declares an ordinary text field and returns a box that shows more than one line.
//
// The definition stays `text_field`, and the component is now the toolkit's `text_input` carrying
// `multiline` rather than a wire type of ours. That type lived one release: it existed because the
// cheap shape of the same addition belonged to whoever owns `text_input`, and it stopped existing
// when they added it (kompot#28).
//
// The price that came with the extension is gone with it. A client that did not know the component
// drew a placeholder and left the field declared and unfillable — the state §9.2 tells servers to
// avoid — and there was no way to say what to draw instead. A client that does not know the flag
// ignores it (§3) and shows an ordinary box holding the same value.
func (b *Builder) MultilineInput(fieldID, label, placeholder, hint string, lines int, rules []Rule) render.Component {
	b.declare(fieldID, textField{Type: "text_field", FieldID: fieldID, Rules: nonNilRules(rules)})
	return render.Multiline(componentID(fieldID), fieldID, label, placeholder)
}
