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
}

type row struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Modifiers []Modifier  `json:"modifiers,omitempty"`
	Children  []Component `json:"children"`
	Spacing   int         `json:"spacing,omitempty"`
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
