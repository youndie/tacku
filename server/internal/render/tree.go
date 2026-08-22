// Package render builds KOMPOT component trees.
//
// The vocabulary is closed and small — this file is nearly all of it — so the types are written out
// rather than generated. What matters is not the shapes but three rules the constructors enforce by
// existing: modifiers are an ordered chain, dimensions are the only numbers, and every node carries
// an identifier.
package render

// Component is a node of the tree. Any of the concrete types below.
type Component any

// Action is what a button does.
type Action any

// Modifier is one link of the chain. Order is significant: padding then background fills the padded
// box, background then padding paints the content and pads outside it.
type Modifier any

type padding struct {
	Type   string `json:"type"`
	All    *int   `json:"all,omitempty"`
	Top    *int   `json:"top,omitempty"`
	Bottom *int   `json:"bottom,omitempty"`
	Start  *int   `json:"start,omitempty"`
	End    *int   `json:"end,omitempty"`
}

type background struct {
	Type  string `json:"type"`
	Color string `json:"color"`
}

type size struct {
	Type     string  `json:"type"`
	Width    *string `json:"width,omitempty"`
	Height   *string `json:"height,omitempty"`
	WidthDp  *int    `json:"widthDp,omitempty"`
	HeightDp *int    `json:"heightDp,omitempty"`
}

type weight struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

func intPtr(v int) *int { return &v }

// Padding pads every side equally.
func Padding(all int) Modifier { return padding{Type: "padding", All: intPtr(all)} }

// PaddingXY pads vertically and horizontally, the way a design usually states it.
func PaddingXY(vertical, horizontal int) Modifier {
	return padding{
		Type: "padding",
		Top:  intPtr(vertical), Bottom: intPtr(vertical),
		Start: intPtr(horizontal), End: intPtr(horizontal),
	}
}

// Background paints the node with a colour token. Tokens are open string keys the client resolves;
// a name it does not know costs a default and a warning, never a broken screen.
func Background(token string) Modifier { return background{Type: "background", Color: token} }

// WidthDp and HeightDp are absolute extents.
//
// They arrived in kompot 0.9.0 after this project reported that SPEC.md promised integer dp while
// the schema carried only Fill and Wrap. A number on an axis outranks a symbolic value on the same
// axis, so the two are never set together here.
func WidthDp(dp int) Modifier  { return size{Type: "size", WidthDp: intPtr(dp)} }
func HeightDp(dp int) Modifier { return size{Type: "size", HeightDp: intPtr(dp)} }

// Weight claims a share of the free space of the parent row or column. Also the only way to push a
// sibling to an edge, there being no alignment in the vocabulary.
func Weight(value float64) Modifier { return weight{Type: "weight", Value: value} }

type column struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Modifiers []Modifier  `json:"modifiers,omitempty"`
	Children  []Component `json:"children"`
	Spacing   int         `json:"spacing,omitempty"`
	Action    Action      `json:"action,omitempty"`
}

type row struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Modifiers []Modifier  `json:"modifiers,omitempty"`
	Children  []Component `json:"children"`
	Spacing   int         `json:"spacing,omitempty"`
	Action    Action      `json:"action,omitempty"`
}

type text struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Modifiers []Modifier `json:"modifiers,omitempty"`
	Text      string     `json:"text"`
	Style     string     `json:"style,omitempty"`
}

type button struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Modifiers []Modifier `json:"modifiers,omitempty"`
	Text      string     `json:"text"`
	Action    Action     `json:"action"`
}

type paginatedList struct {
	Type           string      `json:"type"`
	ID             string      `json:"id"`
	Modifiers      []Modifier  `json:"modifiers,omitempty"`
	InitialItems   []Component `json:"initialItems"`
	LoadMoreAction *loadPage   `json:"loadMoreAction,omitempty"`
	ReloadURL      *string     `json:"reloadUrl,omitempty"`
	EmptyState     Component   `json:"emptyState,omitempty"`
}

// Column and Row take their children last so a call reads like the tree it builds.
func Column(id string, spacing int, modifiers []Modifier, children ...Component) Component {
	return column{Type: "column", ID: id, Modifiers: modifiers, Children: nonNil(children), Spacing: spacing}
}

func Row(id string, spacing int, modifiers []Modifier, children ...Component) Component {
	return row{Type: "row", ID: id, Modifiers: modifiers, Children: nonNil(children), Spacing: spacing}
}

// Opens is a container the whole of which is one target.
//
// It exists as of kompot 0.15. Before it, the vocabulary had no way to say "this row opens that
// thing" — no modifier made a node tappable, a table row was a list of strings, and only a button
// carried an action — so a list of openable things had to be drawn as a list of buttons, one per
// entry. That was written down as a gap rather than worked around, and the answer upstream put an
// action on the container itself.
//
// The button it replaces is gone rather than kept beside it: two ways to open the same thing is one
// more than a reader can be asked to distinguish.
func Opens(container Component, action Action) Component {
	switch value := container.(type) {
	case row:
		value.Action = action
		return value
	case column:
		value.Action = action
		return value
	}
	// Silence here would be a row that quietly does nothing, which is the failure this whole
	// mechanism exists to avoid.
	panic("render: only a row or a column can carry an action")
}

func Text(id, body, style string, modifiers ...Modifier) Component {
	return text{Type: "text", ID: id, Modifiers: modifiers, Text: body, Style: style}
}

func Button(id, label string, action Action, modifiers ...Modifier) Component {
	return button{Type: "button", ID: id, Modifiers: modifiers, Text: label, Action: action}
}

// Spacer is the idiom for pushing a sibling to the edge of a row: an empty column taking the free
// space. There is no alignment modifier, so this is not a trick but the mechanism.
func Spacer(id string) Component { return Column(id, 0, []Modifier{Weight(1)}) }

// Rule is a line of the given thickness, standing in for a border. The vocabulary has none.
func Rule(id string, thicknessDp int, token string, horizontal bool) Component {
	dimension := HeightDp(thicknessDp)
	if !horizontal {
		dimension = WidthDp(thicknessDp)
	}
	return Column(id, 0, []Modifier{dimension, Background(token)})
}

type navigateAction struct {
	Type     string `json:"type"`
	Deeplink string `json:"deeplink"`
}

type loadPage struct {
	URL string `json:"url"`
}

type loadPageAction struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Navigate opens a screen of this application. The deeplink is an application URI and must not be a
// web address — the schema refuses one, which is what stops a server sending a client off-site.
func Navigate(deeplink string) Action { return navigateAction{Type: "navigate", Deeplink: deeplink} }

// LoadPageAt is the polymorphic form, for a button.
func LoadPageAt(url string) Action { return loadPageAction{Type: "load_page", URL: url} }

// PaginatedList holds the first page inline and the address of the next.
//
// loadMoreAction sits in a concrete position rather than a polymorphic one, so it carries no
// discriminator: the client parses that field with a fixed serialiser, and a `type` key there is
// rubbish at best and a parse error at worst.
func PaginatedList(id string, items []Component, nextURL string, empty Component, modifiers ...Modifier) Component {
	list := paginatedList{
		Type: "paginated_list", ID: id, Modifiers: modifiers,
		InitialItems: nonNil(items), EmptyState: empty,
	}
	if nextURL != "" {
		list.LoadMoreAction = &loadPage{URL: nextURL}
	}
	return list
}

// Filtered is a list that reloads from the top when the form around it changes.
//
// `reloadUrl` is the only way a field's value reaches a list: the schema says the form's values go
// to that address as query parameters. Without it a filter is a control that looks like one — the
// server reads `?status=` and nothing ever sends it, and nothing fails.
func Filtered(list Component, reloadURL string) Component {
	value, ok := list.(paginatedList)
	if !ok {
		panic("render: only a paginated list reloads")
	}
	value.ReloadURL = &reloadURL
	return value
}

// PageResponse is the envelope a `page` endpoint answers with.
type PageResponse struct {
	Items          []Component `json:"items"`
	NextLoadAction *loadPage   `json:"nextLoadAction,omitempty"`
}

func Page(items []Component, nextURL string) PageResponse {
	page := PageResponse{Items: nonNil(items)}
	if nextURL != "" {
		page.NextLoadAction = &loadPage{URL: nextURL}
	}
	return page
}

func nonNil(items []Component) []Component {
	if items == nil {
		return []Component{}
	}
	return items
}

type textInput struct {
	Type        string     `json:"type"`
	ID          string     `json:"id"`
	Modifiers   []Modifier `json:"modifiers,omitempty"`
	FieldID     string     `json:"fieldId"`
	Label       string     `json:"label"`
	Placeholder string     `json:"placeholder,omitempty"`
	Mask        string     `json:"mask,omitempty"`
	Secret      bool       `json:"secret,omitempty"`
}

type selectInput struct {
	Type        string         `json:"type"`
	ID          string         `json:"id"`
	Modifiers   []Modifier     `json:"modifiers,omitempty"`
	FieldID     string         `json:"fieldId"`
	Label       string         `json:"label"`
	Options     []SelectOption `json:"options"`
	Placeholder string         `json:"placeholder,omitempty"`
}

type checkboxInput struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Modifiers []Modifier `json:"modifiers,omitempty"`
	FieldID   string     `json:"fieldId"`
	Label     string     `json:"label"`
}

// SelectOption is a choice offered by a select or a radio group. It belongs to the component and
// not to the schema: the same field may be drawn either way.
type SelectOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// The input constructors are unexported from the point of view of a caller building a form: they are
// reached through forms.Builder, which registers the field declaration in the same call. Placing an
// input without declaring its field is then not something anybody has to remember not to do.
func TextInput(id, fieldID, label, placeholder, mask string, secret bool) Component {
	return textInput{
		Type: "text_input", ID: id, FieldID: fieldID, Label: label,
		Placeholder: placeholder, Mask: mask, Secret: secret,
	}
}

func SelectInput(id, fieldID, label, placeholder string, options []SelectOption) Component {
	if options == nil {
		options = []SelectOption{}
	}
	return selectInput{
		Type: "select_input", ID: id, FieldID: fieldID, Label: label,
		Options: options, Placeholder: placeholder,
	}
}

func CheckboxInput(id, fieldID, label string) Component {
	return checkboxInput{Type: "checkbox_input", ID: id, FieldID: fieldID, Label: label}
}

type submitFormAction struct {
	Type   string `json:"type"`
	FormID string `json:"formId"`
}

// SubmitForm hands the form over. Its answer is a KompotAction the client feeds through the same
// chain as any other intent — never a redirect, never an empty 200 (§16.4).
func SubmitForm(formID string) Action { return submitFormAction{Type: "submit_form", FormID: formID} }

type performAction struct {
	Type    string         `json:"type"`
	URL     string         `json:"url"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Perform acts on one item of a list, which is what a button on a card needs and what the protocol
// gained in 0.11 after this project reported it missing.
//
// The payload carries FieldValue values, so it is machine-checkable — and that is also its danger:
// the value hierarchy does not degrade, so a type the client does not know fails the parse of the
// whole screen rather than one button. The payloads built here stay to text values for that reason.
func Perform(url string, payload map[string]any) Action {
	return performAction{Type: "perform", URL: url, Payload: payload}
}

// FieldText is the only value shape this server sends.
func FieldText(text string) map[string]any {
	return map[string]any{"type": "text_value", "text": text}
}

type readOnlyField struct {
	Type       string     `json:"type"`
	ID         string     `json:"id"`
	Modifiers  []Modifier `json:"modifiers,omitempty"`
	Label      string     `json:"label"`
	Value      string     `json:"value"`
	HelperText string     `json:"helperText,omitempty"`
}

// ReadOnlyField shows a value the server has already decided.
//
// It has no fieldId and is declared in no schema: its value arrives finished and never travels back
// in a submit, which is why it is the one input-looking component the form builder does not own.
func ReadOnlyField(id, label, value, helper string, modifiers ...Modifier) Component {
	return readOnlyField{
		Type: "read_only_field", ID: id, Modifiers: modifiers,
		Label: label, Value: value, HelperText: helper,
	}
}

type dateInput struct {
	Type          string     `json:"type"`
	ID            string     `json:"id"`
	Modifiers     []Modifier `json:"modifiers,omitempty"`
	FieldID       string     `json:"fieldId"`
	Label         string     `json:"label"`
	DisplayFormat string     `json:"displayFormat,omitempty"`
	Placeholder   string     `json:"placeholder,omitempty"`
	Hint          string     `json:"hint,omitempty"`
}

// DateInput is the tree half of the deployment's own field type — see forms.Builder.DateInput.
//
// `displayFormat` travels from here because the server owns every string on the screen, and a date
// the client formatted itself would be the one place it did not.
func DateInput(id, fieldID, label, hint string, modifiers ...Modifier) Component {
	return dateInput{
		Type: "date_input", ID: id, Modifiers: modifiers,
		FieldID: fieldID, Label: label, DisplayFormat: dayLayoutPattern, Hint: hint,
	}
}

type multilineInput struct {
	Type        string     `json:"type"`
	ID          string     `json:"id"`
	Modifiers   []Modifier `json:"modifiers,omitempty"`
	FieldID     string     `json:"fieldId"`
	Label       string     `json:"label"`
	Placeholder string     `json:"placeholder,omitempty"`
	Hint        string     `json:"hint,omitempty"`
	MinLines    int        `json:"minLines"`
}

// DefaultLines is how tall a box for prose is when the caller has no opinion.
//
// Lines rather than dp, and that is the one measurement the protocol has no unit for: §5.3 gives it
// exactly one, and the height that matters for text is a count of lines of the reader's own font
// (Q-41). The design asked for this with `text_input [size h 96]`, which is a single-line box 96 dp
// tall — geometry changed, behaviour unchanged.
const DefaultLines = 4

// MultilineInput is the tree half of the deployment's own box for prose — see
// forms.Builder.MultilineInput.
//
// `minLines` is written out even when it equals the client's own default, never omitted. Two
// defaults for one number is how the wire and the screen come to disagree in silence, and the
// server owns every other decision about this screen.
func MultilineInput(id, fieldID, label, placeholder, hint string, lines int, modifiers ...Modifier) Component {
	if lines < 1 {
		lines = DefaultLines
	}
	return multilineInput{
		Type: "multiline_input", ID: id, Modifiers: modifiers,
		FieldID: fieldID, Label: label, Placeholder: placeholder, Hint: hint, MinLines: lines,
	}
}
