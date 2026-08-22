package tacku.app

import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.remember
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
            content = content,
        )
    }
}

/** The one this application uses; the screenshots pass their own, carrying the font they ship. */
@Composable
fun rememberTackuDesignSystem(): TackuDesignSystem = remember { TackuDesignSystem() }
