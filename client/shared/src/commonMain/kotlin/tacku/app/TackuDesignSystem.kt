package tacku.app

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.youndie.kompot.ColorToken
import io.github.youndie.kompot.KompotDesignSystem
import io.github.youndie.kompot.KompotSurface
import io.github.youndie.kompot.KompotSurfaceRoles
import io.github.youndie.kompot.SurfaceRole
import io.github.youndie.kompot.TypographyToken

/**
 * The token set, and this file is where it is declared.
 *
 * §6 puts it here rather than on the server: the protocol carries **names**, the design system
 * decides what they mean, and the server is required to use these names rather than invent its own.
 * Which is why the same list exists twice — as constants here and as constants in the Go renderer —
 * and why a test compares them: a name only one side knows costs a default and a warning, so the
 * drift is silent and looks like a design decision.
 *
 * Two rules of the set are structural rather than editorial:
 *
 *  - **A colour token is only ever a background or a gradient.** Text colour lives inside the
 *    typography token, because `text` carries a style and no colour at all.
 *  - **`agent` paints nothing but the provenance stripe.** The moment it appears on a decorative
 *    element the signal stops meaning anything, and that is checked on the server rather than
 *    remembered here.
 */
class TackuDesignSystem(
    private val dark: Boolean = true,
    /**
     * What every style is built on top of.
     *
     * It exists because a `TextStyle` with no font family is drawn in whatever font the machine
     * happens to have, and a screenshot of that is a picture of the machine — which is what this
     * product was, for months, in both places at once. The design is set in IBM Plex Sans and the
     * client drew in whatever the system offered, so the running product and the mockup were never
     * the same picture and no screenshot could have said so.
     *
     * The default carries no family and that is now deliberate rather than an oversight: the
     * typeface arrives over the network in a browser, so it cannot be a constant. Exactly two places
     * construct a design system that will be looked at — [App], which waits for the family before it
     * draws anything, and the screenshot harness, which passes the same files with normalised
     * metrics. Everything else that constructs one is asking it for token names, where the font is
     * not part of the question.
     */
    private val base: TextStyle = TextStyle.Default,
) : KompotDesignSystem {
    @Composable
    override fun resolveColor(token: ColorToken): Color =
        colors[token.key] ?: MaterialTheme.colorScheme.surface.also { warn("colour", token.key) }

    @Composable
    override fun resolveTypography(token: TypographyToken): TextStyle =
        typography[token.key] ?: MaterialTheme.typography.bodyMedium.also { warn("typography", token.key) }

    /**
     * The third key, and the one that ends a whole family of workarounds.
     *
     * A colour token answers a name the **server** sent; this answers a role the **client** knows —
     * `button`, `field`, `read_only_field`, `container`, and a button's variant beside them. So the
     * appearance of a control stays off the wire, which is the property worth protecting, while the
     * product still gets to decide what a control looks like. Until 0.22 there was no such place at
     * all: the shape of a Material button comes from `ButtonDefaults` rather than the theme, so
     * zeroing every `Shapes` slot changed nothing, and the only remedy was to draw the button
     * ourselves and lose everything the toolkit's renderer knew about the modifier chain (Q-58).
     *
     * All four slots are filled every time, deliberately. A surface with a container and no content
     * colour leaves whatever text colour was already in scope on top of a background chosen without
     * it — that is how this product drew a consent checkbox in black on near-black, and how the
     * toolkit's own first screenshot of this feature caught the same shape of mistake.
     */
    @Composable
    override fun resolveSurface(role: SurfaceRole): KompotSurface =
        when (role.key) {
            primaryButton -> surface(container = color("accent"), content = resolved("button_primary"))
            "button" -> surface(container = Color.Transparent, content = resolved("button_quiet"))
            "field" -> surface(container = color("surface_field"), content = resolved("body"))

            // Its own role rather than the field's, because it exists to say "this is a value, not
            // an input" and used to draw as the editable box beside it, which says the opposite.
            // Flat and unbordered: a value is text on a block, not a control.
            "read_only_field" -> surface(container = color("surface_block"), content = resolved("value"))

            else -> surface(container = color("surface_block"), content = resolved("body"))
        }

    // Matched by key rather than by identity: the role for a variant is composed by the toolkit, so
    // asking it for the key is the only way to be sure of the one we answer to.
    private val primaryButton = KompotSurfaceRoles.button(VARIANT_PRIMARY).key

    /**
     * Square, and never outlined.
     *
     * Both come from the same line of the design: no rounding, no borders, no shadows. The outline
     * is transparent rather than absent because the slot exists; leaving it to a default would be
     * the toolkit deciding a question the design has already answered.
     */
    private fun surface(
        container: Color,
        content: Color,
    ) = KompotSurface(
        shape = square,
        container = container,
        content = content,
        outline = Color.Transparent,
    )

    /**
     * The same tokens, in the shape Material asks for.
     *
     * Without this the product has two palettes and only one of them is its own. A design system
     * answers the names the SERVER sends — a background, a typography token — and every control the
     * toolkit draws for itself takes its colour from `MaterialTheme`, which was left at the baseline
     * scheme. The sign-in button showed both at once: this deployment's accent painted as a square
     * behind, and Material's default lavender pill drawn on top of it. Measured from a screenshot of
     * the running application: #5069D6 at the corner, a baseline purple in the middle.
     *
     * Nothing else would have caught it. The screenshot tests build their own harness with the same
     * baseline theme, so they agreed with the application about a colour neither of them had; the
     * conformance walk reads bodies, and the body was correct. Only launching it showed two colours
     * where the design has one.
     */
    fun materialColors(): ColorScheme {
        val base = if (dark) darkColorScheme() else lightColorScheme()
        return base.copy(
            primary = color("accent"),
            onPrimary = Color(0xFFFFFFFF),
            secondary = color("agent"),
            background = color("surface"),
            onBackground = resolved("body"),
            surface = color("surface_block"),
            onSurface = resolved("body"),
            surfaceVariant = color("surface_field"),
            onSurfaceVariant = resolved("meta"),
            outline = color("divider"),
            error = resolved("error"),
        )
    }

    /**
     * Rectangles, because the vocabulary has no corners.
     *
     * The modifier list is closed at background, gradient, padding, size and weight — there is no
     * radius, no border and no shadow, so a server cannot ask for a rounded anything and the design
     * was written around that: "скруглений, рамок и теней нет". A Material control rounds itself by
     * default, which is the client quietly answering a question the protocol does not let anybody
     * ask.
     */
    fun materialShapes(): Shapes =
        Shapes(
            extraSmall = square,
            small = square,
            medium = square,
            large = square,
            extraLarge = square,
        )

    // Zero rather than RectangleShape, because Material's shape slots are typed to corner-based
    // shapes: the type says a corner exists and the value says it has no radius.
    private val square = RoundedCornerShape(0.dp)

    private fun color(key: String): Color = colors[key] ?: Color.Magenta

    private fun resolved(key: String): Color = typography[key]?.color ?: Color.Unspecified

    private val colors: Map<String, Color> =
        if (dark) {
            mapOf(
                "surface" to Color(0xFF101114),
                "surface_block" to Color(0xFF191B1F),
                "surface_field" to Color(0xFF23262B),
                "surface_selected" to Color(0xFF2C313A),
                "accent" to Color(0xFF4869DB),
                "agent" to Color(0xFFAE8ADD),
                "agent_quiet" to Color(0xFF2E240F),
                "danger" to Color(0xFF3B2124),
                "notice" to Color(0xFF2C2717),
                "divider" to Color(0xFF292D33),
                "status_active" to Color(0xFF1D3346),
                "status_done" to Color(0xFF1E3329),
            )
        } else {
            mapOf(
                "surface" to Color(0xFFF4F5F7),
                "surface_block" to Color(0xFFFFFFFF),
                "surface_field" to Color(0xFFEDEFF2),
                "surface_selected" to Color(0xFFDCE3EE),
                "accent" to Color(0xFF3B57C4),
                "agent" to Color(0xFF8259C2),
                "agent_quiet" to Color(0xFFFBF1DE),
                "danger" to Color(0xFFFBE7E6),
                "notice" to Color(0xFFFCF2D9),
                "divider" to Color(0xFFD9DDE4),
                "status_active" to Color(0xFFDCE9F7),
                "status_done" to Color(0xFFDCEDE1),
            )
        }

    // A typography token carries size, weight **and** colour, because the protocol gives `text` no
    // colour field. A red error message is therefore a token and not a text plus a colour.
    private val typography: Map<String, TextStyle> =
        mapOf(
            "display" to style(28, FontWeight.SemiBold, if (dark) 0xFFF2F3F5 else 0xFF14161A),
            "title" to style(20, FontWeight.SemiBold, if (dark) 0xFFF2F3F5 else 0xFF14161A),
            "subtitle" to style(15, FontWeight.SemiBold, if (dark) 0xFF9AA1AC else 0xFF5C6470),
            "body" to style(14, FontWeight.Normal, if (dark) 0xFFE3E5E9 else 0xFF24282F),
            "body_muted" to style(14, FontWeight.Normal, if (dark) 0xFF9AA1AC else 0xFF5C6470),
            "value" to style(14, FontWeight.Medium, if (dark) 0xFFF2F3F5 else 0xFF14161A),
            "label" to style(12, FontWeight.Medium, if (dark) 0xFF9AA1AC else 0xFF4F5766),
            "meta" to style(12, FontWeight.Normal, if (dark) 0xFF949BA6 else 0xFF5A6270),
            "meta_agent" to style(12, FontWeight.Medium, if (dark) 0xFFC7A6E6 else 0xFF6B45A8),
            BUTTON_PRIMARY_STYLE to style(14, FontWeight.SemiBold, 0xFFFFFFFF),
            "button_quiet" to style(14, FontWeight.Medium, if (dark) 0xFFA9B6E8 else 0xFF3B57C4),
            "error" to style(13, FontWeight.Normal, if (dark) 0xFFF0908C else 0xFFB3261E),
            "notice" to style(13, FontWeight.Medium, if (dark) 0xFFE3B665 else 0xFF7A5205),
            // The rail. The current item is heavier and brighter than its neighbours, because a
            // background alone was not enough to say "you are here" — it read as a decoration.
            "nav" to style(14, FontWeight.Normal, if (dark) 0xFFA8B0BC else 0xFF4F5766),
            "nav_current" to style(14, FontWeight.SemiBold, if (dark) 0xFFFFFFFF else 0xFF14161A),
        )

    /**
     * The same tokens, in the slots Material asks for.
     *
     * It exists because the toolkit's own controls take their text from `MaterialTheme.typography`,
     * and the product handed them the untouched default: a placeholder came out at Material's 16sp
     * in whatever font the machine had, beside a value at this system's 14sp in the product's. The
     * two sat in the same field and did not match, which is a design defect nothing in the wire
     * could have caused and nothing in the design system could have fixed while the theme was
     * carrying somebody else's numbers.
     *
     * Only the slots a control reaches for are mapped. The rest keep Material's sizes and inherit
     * the family from [base], which is enough: a slot nothing draws from is a slot nothing sees.
     */
    fun materialTypography(): Typography =
        Typography(
            displayLarge = styleOf("display"),
            displayMedium = styleOf("display"),
            displaySmall = styleOf("display"),
            headlineLarge = styleOf("title"),
            headlineMedium = styleOf("title"),
            headlineSmall = styleOf("title"),
            titleLarge = styleOf("title"),
            titleMedium = styleOf("subtitle"),
            titleSmall = styleOf("subtitle"),
            bodyLarge = styleOf("body"),
            bodyMedium = styleOf("body"),
            bodySmall = styleOf("meta"),
            // The quiet button's metrics, not the primary one's, because Material gives both the
            // same slot and only one can have it. Colour already tells them apart — it comes from
            // the surface role — so the shared half is weight, and the emphasised button is
            // emphasised by its fill. Mapping the primary style here turned every quiet label bold
            // and white: "Mark all as seen" shouted from a corner of the catch-up screen.
            labelLarge = styleOf("button_quiet"),
            labelMedium = styleOf("label"),
            labelSmall = styleOf("meta"),
        )

    /**
     * One token as a style, with its colour left out.
     *
     * Size, weight and family are the token's; colour is not, because these styles are handed to
     * Material and Material's controls take their colour from the surface the design system already
     * answers for. Carrying it here painted a quiet button's label in the primary button's white:
     * "Mark all as seen" turned bold and white the moment this mapping arrived, which is the whole
     * argument for who owns what in one screenshot.
     */
    private fun styleOf(key: String): TextStyle = (typography[key] ?: base).copy(color = Color.Unspecified)

    private fun style(
        size: Int,
        weight: FontWeight,
        color: Long,
    ) = base.copy(fontSize = size.sp, fontWeight = weight, color = Color(color))

    /**
     * A name the design system does not know costs a default and a line in the log — never a broken
     * screen (§6). Which is exactly why the drift between the two lists has to be caught by a test:
     * on screen it looks like a deliberate lack of emphasis.
     */
    private fun warn(
        kind: String,
        token: String,
    ) {
        println("tacku: unknown $kind token \"$token\"; using a default")
    }

    companion object {
        /** The variant this server sends for the one button on a screen that is the action. */
        const val VARIANT_PRIMARY = "primary"

        /** The typography token a primary button's label is set in — named so that a caller can ask for it. */
        const val BUTTON_PRIMARY_STYLE = "button_primary"

        /** The names, which are the half the server has to agree with. */
        val colorTokens: List<String> = TackuDesignSystem().colors.keys.sorted()

        val typographyTokens: List<String> = TackuDesignSystem().typography.keys.sorted()
    }
}
