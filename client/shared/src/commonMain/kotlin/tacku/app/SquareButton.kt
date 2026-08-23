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
import io.github.youndie.kompot.KompotSurfaceRoles
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.TypographyToken

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
    val design = LocalKompotDesignSystem.current
    val surface = design.resolveSurface(KompotSurfaceRoles.button(TackuDesignSystem.VARIANT_PRIMARY))

    // The label's style comes from the design system, like every other piece of text here.
    //
    // It used to be a bare `TextStyle(color = …)`, which names no font family — so the one control
    // the design cares most about was drawn in whatever the machine had installed, while everything
    // around it was set in the product's typeface. It was invisible on any single machine: the
    // fallback is a reasonable-looking sans, and a picture of it is stable. Two machines are what
    // told them apart.
    val labelStyle =
        design
            .resolveTypography(
                TypographyToken(TackuDesignSystem.BUTTON_PRIMARY_STYLE),
            ).copy(color = surface.content)

    Box(
        Modifier
            // The shape is optional in the surface: a design system may decline to name one, and
            // then the corner is whoever draws it. Ours always answers, and this says what happens
            // if it ever stops — a rectangle, never a default from somewhere else.
            .background(surface.container, surface.shape ?: RectangleShape)
            .clickable { onClick() }
            .padding(padding),
    ) {
        Text(label, style = labelStyle)
    }
}
