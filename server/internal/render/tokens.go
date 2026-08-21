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
