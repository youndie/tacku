package tacku.app

import androidx.compose.foundation.LocalIndication
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import io.github.youndie.kompot.ColorToken
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.TypographyToken

/**
 * Everything a screen of this product needs to be drawn in this product's colours, in one place.
 *
 * There are three ways a colour reaches the screen and the design system answers only the first:
 *
 *  1. the server names a token — resolved by [TackuDesignSystem];
 *  2. a Material control paints itself — takes the `ColorScheme`, so that is built from the same
 *     tokens;
 *  3. a control draws text without saying what colour — takes `LocalContentColor`.
 *
 * The third was found by photographing a real form: the consent checkbox's label — "Let my agent
 * keep this task up to date", the sentence that says what agreeing means — was drawn in pure black
 * on the near-black background, 1.06:1, invisible. `LocalContentColor` defaults to black and is
 * normally set by a Material `Surface`; nothing here was a `Surface`, so every control that colours
 * its own text got black. It cost nothing to have and nothing said so: the screen rendered, the
 * checkbox worked, and the text was there.
 *
 * Which is why this is a composable both the application and the screenshots call, rather than a
 * block of setup each of them writes. Two copies of it is how the pictures came to be of the
 * toolkit rather than of the product.
 */
@Composable
fun TackuTheme(
    design: TackuDesignSystem,
    typography: Typography = Typography(),
    content: @Composable () -> Unit,
) {
    MaterialTheme(
        colorScheme = design.materialColors(),
        shapes = design.materialShapes(),
        typography = typography,
    ) {
        CompositionLocalProvider(
            LocalKompotDesignSystem provides design,
            LocalContentColor provides design.resolveTypography(TypographyToken("body")).color,
            // Every pressable thing in the product highlights the same way, and highlights all of
            // itself. Material's ripple is round, which on a design with no corners is wrong twice,
            // and it arrives in Material's colours rather than these.
            LocalIndication provides
                TackuIndication(
                    hovered = design.resolveColor(ColorToken("surface_selected")).copy(alpha = 0.55f),
                    pressed = design.resolveColor(ColorToken("surface_selected")),
                ),
        ) {
            // The window is painted here rather than by whatever happens to be on it.
            //
            // Every screen paints `surface` over its whole area, so for a while nothing else needed
            // to. Then the server went away: the failure message is a line of text and nothing else,
            // and a line of text over an unpainted window is a white screen with a pale line on it.
            // The states that are not screens — loading, failed — are exactly the ones nobody looks
            // at until something is already wrong.
            Box(Modifier.fillMaxSize().background(design.materialColors().background)) {
                content()
            }
        }
    }
}
