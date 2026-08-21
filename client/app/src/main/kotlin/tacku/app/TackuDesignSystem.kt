package tacku.app

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import io.github.youndie.kompot.ColorToken
import io.github.youndie.kompot.KompotDesignSystem
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
     * happens to have, and a screenshot of that is a picture of the machine. The screenshot harness
     * passes a font it carries with it; the product leaves this alone and gets the system font,
     * which is what a desktop application should look like.
     */
    private val base: TextStyle = TextStyle.Default,
) : KompotDesignSystem {
    @Composable
    override fun resolveColor(token: ColorToken): Color =
        colors[token.key] ?: MaterialTheme.colorScheme.surface.also { warn("colour", token.key) }

    @Composable
    override fun resolveTypography(token: TypographyToken): TextStyle =
        typography[token.key] ?: MaterialTheme.typography.bodyMedium.also { warn("typography", token.key) }

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
            "button_primary" to style(14, FontWeight.SemiBold, 0xFFFFFFFF),
            "button_quiet" to style(14, FontWeight.Medium, if (dark) 0xFFA9B6E8 else 0xFF3B57C4),
            "error" to style(13, FontWeight.Normal, if (dark) 0xFFF0908C else 0xFFB3261E),
            "notice" to style(13, FontWeight.Medium, if (dark) 0xFFE3B665 else 0xFF7A5205),
        )

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
        System.err.println("tacku: unknown $kind token \"$token\"; using a default")
    }

    companion object {
        /** The names, which are the half the server has to agree with. */
        val colorTokens: List<String> = TackuDesignSystem().colors.keys.sorted()

        val typographyTokens: List<String> = TackuDesignSystem().typography.keys.sorted()
    }
}
