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

// Modifier is one link of the chain. Order is significant: padding then background paints inside the
// padding and leaves the rest as a margin, background then padding paints the whole node and insets
// the content.
type Modifier any

// canonical puts the paint before the padding, and it exists because every screen of this product
// had it the other way round.
//
// The two orders both mean something, and this product wants one of them everywhere: a card is a
// block of colour with its text inset, a screen is a surface that reaches the window's edge. What
// was emitted instead was 52 nodes out of 53 with padding first — so each card's colour stopped
// short of its own text, and each screen's surface stopped 32dp short of the window, showing the
// bare white frame of the window behind it. One mistake, six symptoms, and every one of them looked
// like a different bug about spacing.
//
// It is done here rather than at the call sites because a call site is a place to forget. Where a
// margin really is wanted — a gap between the items of a `paginated_list`, which has no `spacing` of
// its own — it is a node with padding and no background wrapped around one with both, and that
// wrapper is unaffected by this.
func canonical(modifiers []Modifier) []Modifier {
	if len(modifiers) < 2 {
		return modifiers
	}

	ordered := make([]Modifier, 0, len(modifiers))
	for _, modifier := range modifiers {
		if paints(modifier) {
			ordered = append(ordered, modifier)
		}
	}
	if len(ordered) == 0 || len(ordered) == len(modifiers) {
		return modifiers
	}
	for _, modifier := range modifiers {
		if !paints(modifier) {
			ordered = append(ordered, modifier)
		}
	}
	return ordered
}

func paints(modifier Modifier) bool {
	switch modifier.(type) {
	// The vocabulary also has `gradient`, and it would belong here; this server does not build one
	// yet, and a case for a type that does not exist is a claim rather than a rule.
	case background:
		return true
	}
	return false
}

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
func WidthDp(dp int) Modifier { return size{Type: "size", WidthDp: intPtr(dp)} }

// FillWidth makes a node as wide as its parent allows.
//
// Found late and the hard way. `size` carries `width`/`height` of type `SizeType`, which is
// `Fill | Wrap` — and this file declared the fields (`Width *string`) without ever setting one, so
// every "make it span" was done with a number in dp or not at all. What it looked like on screen: a
// card's colour ending where its longest line ended, a menu item highlighted only as wide as its
// own word. What it looked like in the journal: a report filed upstream claiming the vocabulary had
// no way to say this (Q-59, withdrawn).
func FillWidth() Modifier { return size{Type: "size", Width: fill()} }

// FillHeight makes a node as tall as its parent allows.
func FillHeight() Modifier { return size{Type: "size", Height: fill()} }

func fill() *string {
	value := "Fill"
	return &value
}
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
	MaxLines  *int       `json:"maxLines,omitempty"`
	Ellipsis  bool       `json:"ellipsis,omitempty"`
}

type button struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Modifiers []Modifier `json:"modifiers,omitempty"`
	Text      string     `json:"text"`
	Action    Action     `json:"action"`
	Variant   string     `json:"variant,omitempty"`
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
	return column{Type: "column", ID: id, Modifiers: canonical(modifiers), Children: nonNil(children), Spacing: spacing}
}

func Row(id string, spacing int, modifiers []Modifier, children ...Component) Component {
	return row{Type: "row", ID: id, Modifiers: canonical(modifiers), Children: nonNil(children), Spacing: spacing}
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
	return text{Type: "text", ID: id, Modifiers: canonical(modifiers), Text: body, Style: style}
}

func Button(id, label string, action Action, modifiers ...Modifier) Component {
	return button{Type: "button", ID: id, Modifiers: canonical(modifiers), Text: label, Action: action}
}

// VariantPrimary is the one button on a screen that is the action, as opposed to the way out.
//
// It exists on the wire from kompot 0.22 and it is content rather than theme: which button is the
// main one is decided by whoever wrote the screen. Before it, this server said the same thing by
// painting the button with the accent token and the client inferred emphasis from the presence of
// a background — a guess that happened to be shared by both halves because the same person wrote
// them. A deployment that did not share it would have drawn "Cancel" exactly like "Submit".
const VariantPrimary = "primary"

// PrimaryButton is the emphasised button. The colour is the client's business now: the variant names
// the role and the design system answers it, so nothing about appearance travels.
func PrimaryButton(id, label string, action Action, modifiers ...Modifier) Component {
	return button{
		Type: "button", ID: id, Modifiers: canonical(modifiers),
		Text: label, Action: action, Variant: VariantPrimary,
	}
}

// Spacer is the idiom for pushing a sibling to the edge of a row: an empty column taking the free
// space. There is no alignment modifier, so this is not a trick but the mechanism.
func Spacer(id string) Component { return Column(id, 0, []Modifier{Weight(1)}) }

// ItemGapDp is half the space between two cards of a list: each carries it above and below.
const ItemGapDp = 6

// PaddingStart pads one side, which is what makes a stripe possible.
func PaddingStart(dp int) Modifier { return padding{Type: "padding", Start: intPtr(dp)} }

// Marked wraps a card so that a stripe of `token` runs down its left edge, exactly as tall as the
// card turns out to be.
//
// The obvious construction — a three-point-wide empty column beside the body — paints nothing, and
// did so on every card of every screen for the life of this project. An empty column has no height,
// and the signal this product exists to carry was therefore absent; absent looks exactly like "a
// person did this". The picture guarding it passed all along, because it asked whether an agent's
// row and a person's row stayed *distinguishable* and they did — by the colour of the meta line, not
// by the device the design chose.
//
// Asking for the height does not work either, and that was worth measuring before giving up on it:
// `height: Fill` resolves against the constraint coming into the row rather than the height of the
// sibling, so the stripe takes the whole screen; `Wrap` on the row changes nothing; an explicit
// `heightDp` is a guess at how many lines the title wrapped to.
//
// So the stripe is not a node at all. The outer node is painted with the mark and inset from the
// start by three points; the inner node paints everything else over it. What is left showing is a
// stripe the exact height of the card, because it *is* the card.
func Marked(id, token string, body Component) Component {
	return Column(id, 0, []Modifier{Background(token), FillWidth(), PaddingStart(StripeDp)}, body)
}

// idOf is the identifier a node carries, which is what an update frame has to name.
//
// Every component of this vocabulary has one, so a node with no identifier is a node this package
// did not build — and the panic says so rather than sending a frame that names nothing.
func idOf(component Component) string {
	switch value := component.(type) {
	case column:
		return value.ID
	case row:
		return value.ID
	case text:
		return value.ID
	case button:
		return value.ID
	case paginatedList:
		return value.ID
	}
	panic("render: this component carries no identifier, so nothing can address it")
}

// Spaced puts a gap around a list item, and it is a whole extra node because there is nowhere else
// to put one.
//
// `column` and `row` carry a `spacing`; `paginated_list` does not (checked in
// `kompot-standard.schema.json`: `initialItems`, `loadMoreAction`, `reloadUrl`, `emptyState` and the
// modifiers, no spacing). So the only space available between two items is a margin, and a margin is
// padding on a node with nothing to paint. Putting that padding on the card itself is what this
// server did until now, and then the card's colour stopped short of its own text: one node cannot be
// both separated from its neighbour and padded inside. Reported upstream.
func Spaced(id string, child Component) Component {
	return Column(id+"-gap", 0, []Modifier{FillWidth(), PaddingXY(ItemGapDp, 0)}, child)
}

// Rule is a line of the given thickness, standing in for a border. The vocabulary has none.
func Rule(id string, thicknessDp int, token string, horizontal bool) Component {
	// Thick one way and filled the other. A rule is an empty column, and an empty column has no
	// size of its own in either direction: giving it a width leaves it nothing tall, so it painted
	// nothing at all — the same way the provenance stripe painted nothing, and just as invisible.
	//
	// One pixel is below the threshold at which a person notices an absence, so this one survived
	// every look at the screen and was found by counting pixels along a row: after the navigation
	// rail came the surface, with no line between them.
	dimensions := []Modifier{HeightDp(thicknessDp), FillWidth()}
	if !horizontal {
		dimensions = []Modifier{WidthDp(thicknessDp), FillHeight()}
	}
	return Column(id, 0, append(dimensions, Background(token)))
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
		Type: "paginated_list", ID: id, Modifiers: canonical(modifiers),
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
	Multiline   bool       `json:"multiline,omitempty"`
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

// Multiline is a text input that shows more than one line of what is typed into it.
//
// It was a wire type of this deployment's own for exactly one release. The type existed only
// because the cheap shape of the same addition — an optional flag on `text_input` — belongs to
// whoever owns the type, and asking for it is what closed the gap: kompot 0.21 carries `multiline`,
// so a whole component of ours would now be a second way to say something the vocabulary says.
//
// What that buys is not tidiness. An unfamiliar component costs a placeholder in a form whose field
// stays declared and unfillable (§9.2); an unfamiliar *flag* costs nothing at all — §3 says a reader
// ignores what it does not know, so a client released before this draws an ordinary one-line box
// with the same value in it.
func Multiline(id, fieldID, label, placeholder string) Component {
	return textInput{
		Type: "text_input", ID: id, FieldID: fieldID, Label: label,
		Placeholder: placeholder, Multiline: true,
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

// FieldEntity is the shape a chosen option travels in, which is not the shape a typed string
// travels in. A select answers with an identity, and the server reads it as one.
//
// The title is required by the contract and is not decoration: a value that carries only an
// identifier cannot be shown back to a person by anything that receives it. Omitting it made the
// whole screen fail to decode — the client refuses the response rather than the field, which is
// §2.2's asymmetry doing exactly what it says it does.
func FieldEntity(id, title string) map[string]any {
	return map[string]any{"type": "entity_value", "id": id, "title": title}
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
		Type: "read_only_field", ID: id, Modifiers: canonical(modifiers),
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
		Type: "date_input", ID: id, Modifiers: canonical(modifiers),
		FieldID: fieldID, Label: label, DisplayFormat: dayLayoutPattern, Hint: hint,
	}
}

// DefaultLines is how tall a box for prose is when the caller has no opinion.
//
// Lines rather than dp, and that is the one measurement the protocol has no unit for: §5.3 gives it
// exactly one, and the height that matters for text is a count of lines of the reader's own font
// (Q-41). The design asked for this with `text_input [size h 96]`, which is a single-line box 96 dp
// tall — geometry changed, behaviour unchanged.
const DefaultLines = 4

type wizardScreen struct {
	Type        string     `json:"type"`
	ID          string     `json:"id"`
	Modifiers   []Modifier `json:"modifiers,omitempty"`
	FormID      string     `json:"formId"`
	StepID      string     `json:"stepId"`
	StepIndex   int        `json:"stepIndex"`
	TotalSteps  *int       `json:"totalSteps"`
	CanGoBack   bool       `json:"canGoBack"`
	Content     Component  `json:"content"`
	FinishLabel string     `json:"finishLabel,omitempty"`
}

// WizardScreen wraps one step of a multi-step flow.
//
// Everything it carries besides the content is bookkeeping the client draws for itself: the step
// counter, the back button and its absence, the finish button (SPEC.md §11.1). None of that goes
// into the content — a step drawing its own header and its own Next would show both twice, which is
// what the first round of the design review found in the mock-up.
//
// totalSteps is a pointer because null is a value the protocol gives a meaning to: under branching
// the length of a particular walk is not known in advance, and the client then shows the current
// step alone (§11.2). Omitting the field and sending null are not the same statement.
// WizardScreen wraps one step, and names its own finishing button.
//
// The chrome — Next, Back, Finish — is drawn by the client, so until kompot 0.21 the last button of
// every flow in every build read the same word. That is tolerable while the last step creates
// something and not tolerable when it is irreversible: "Finish" under a step that deletes a board
// looks harmless, which is the cost. `finishLabel` was asked for on those grounds and arrived; an
// empty one leaves the client's own wording, so a reader released earlier is not affected.
func WizardScreen(id, formID, stepID string, stepIndex int, totalSteps *int, canGoBack bool, finishLabel string, content Component) Component {
	return wizardScreen{
		Type: "wizard_screen", ID: id,
		FormID: formID, StepID: stepID, StepIndex: stepIndex, TotalSteps: totalSteps,
		CanGoBack: canGoBack, Content: content, FinishLabel: finishLabel,
	}
}

// Steps names a walk whose length is known in advance. The pointer is what the wire asks for; a call
// site in the middle of a tree should not have to take an address to say "two".
func Steps(count int) *int { return &count }
