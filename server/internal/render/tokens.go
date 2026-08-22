package render

// The design system's token names.
//
// Duplicated here from docs/design/design-spec-tokens.md, and that duplication is the whole of
// B-24: the set is declared by the client and published as constants, and until it is, these two
// lists can drift without anything noticing. A name the client does not know does not break a
// screen — it costs a default and a warning — which is exactly why the drift would be silent.
const (
	ColorSurface         = "surface"
	ColorSurfaceBlock    = "surface_block"
	ColorSurfaceField    = "surface_field"
	ColorSurfaceSelected = "surface_selected"
	ColorAccent          = "accent"
	ColorAgent           = "agent"
	ColorAgentQuiet      = "agent_quiet"
	ColorDanger          = "danger"
	ColorNotice          = "notice"
	ColorDivider         = "divider"
	ColorStatusActive    = "status_active"
	ColorStatusDone      = "status_done"
)

const (
	TextDisplay   = "display"
	TextTitle     = "title"
	TextSubtitle  = "subtitle"
	TextBody      = "body"
	TextBodyMuted = "body_muted"
	TextValue     = "value"
	TextLabel     = "label"
	TextMeta      = "meta"
	TextMetaAgent = "meta_agent"
	TextError     = "error"
	TextNotice    = "notice"

	// The rail, and the one item of it a person is standing on.
	//
	// Two tokens rather than one style with a flag: the difference is what the screen is for.
	// Everything else on the rail is a way out; this one is where you already are, and the design
	// says so in weight and colour rather than in a background alone.
	TextNav        = "nav"
	TextNavCurrent = "nav_current"

	// The label of a move on a card: a line you press, not a control. The design asked for a link
	// rather than a button there — a board of six cards is six buttons otherwise, and none of them
	// is the action of the screen.
	TextButtonQuiet = "button_quiet"
)

// StripeDp is the width of the provenance stripe, and it is three in both themes.
//
// The design measured that the light theme needs three; the dark one is no worse for it. The reason
// it is not two-in-dark-and-three-in-light is mechanical: a colour survives a theme change because
// the client resolves the token, and a dimension does not, because the server builds the tree and
// does not know the theme.
const StripeDp = 3

// RuleDp is the thickness of a line standing in for a border.
const RuleDp = 1

// ColorTokens and TypographyTokens are what this server actually sends, held against the set the
// client declares.
//
// They are the constants rather than fresh string literals on purpose: the comparison that matters
// is between the two languages, and a name repeated here would only test whether somebody typed it
// twice correctly. Renaming a constant therefore fails the check, which is the case worth catching —
// a token this server sends and the client has never heard of resolves to a default and says so in a
// log nobody reads.
func ColorTokens() []string {
	return []string{
		ColorSurface, ColorSurfaceBlock, ColorSurfaceField, ColorSurfaceSelected,
		ColorAccent, ColorAgent, ColorAgentQuiet, ColorDanger, ColorNotice,
		ColorDivider, ColorStatusActive, ColorStatusDone,
	}
}

func TypographyTokens() []string {
	return []string{
		TextDisplay, TextTitle, TextSubtitle, TextBody, TextBodyMuted, TextValue,
		TextLabel, TextMeta, TextMetaAgent, TextError, TextNotice,
		TextNav, TextNavCurrent,
		// The two the server never chooses: a button carries no style field, so which of them a
		// label is set in is decided by the design system from whether the button has a background.
		// Listed because the client declares them, and an unlisted name would read as drift.
		"button_primary", TextButtonQuiet,
	}
}
