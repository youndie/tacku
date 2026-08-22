package tacku.app

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.text.TextStyle
import io.github.youndie.kompot.KompotSurfaceRoles
import io.github.youndie.kompot.LocalKompotDesignSystem

/**
 * The button the date extension draws, and the only one this client draws at all.
 *
 * Its bigger sibling is gone: from kompot 0.22 the toolkit's own `button` asks the design system for
 * the `button` surface, so replacing that renderer — and losing everything it knew about the
 * modifier chain — is no longer the price of a square corner (Q-58, kompot#33).
 *
 * This one survives because the date field is our component: nothing in the toolkit knows that four
 * chips are a way of naming a day. It takes its appearance from the same place the toolkit's button
 * does, so a chip and a button cannot drift apart without somebody changing the answer for both.
 */
@Composable
fun SquareButton(
    label: String,
    padding: PaddingValues,
    onClick: () -> Unit,
) {
    val surface =
        LocalKompotDesignSystem.current.resolveSurface(
            KompotSurfaceRoles.button(TackuDesignSystem.VARIANT_PRIMARY),
        )

    Box(
        Modifier
            // The shape is optional in the surface: a design system may decline to name one, and
            // then the corner is whoever draws it. Ours always answers, and this says what happens
            // if it ever stops — a rectangle, never a default from somewhere else.
            .background(surface.container, surface.shape ?: RectangleShape)
            .clickable { onClick() }
            .padding(padding),
    ) {
        Text(label, style = TextStyle(color = surface.content))
    }
}
