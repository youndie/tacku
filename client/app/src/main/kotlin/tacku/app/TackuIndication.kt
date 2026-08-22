package tacku.app

import androidx.compose.foundation.IndicationNodeFactory
import androidx.compose.foundation.interaction.HoverInteraction
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.PressInteraction
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.node.DelegatableNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.node.invalidateDraw
import kotlinx.coroutines.launch

/**
 * What a pressable thing does under the pointer, across the whole of itself.
 *
 * Material's own indication is a ripple, and a ripple is round — on a product whose design has no
 * corners that is the wrong shape twice over. It also arrives with Material's colours rather than
 * this product's.
 *
 * The reason it is a whole indication rather than a colour somewhere: a hover has to cover the
 * *control*, and until now it covered whatever drew itself. A menu item is a row with a label inside
 * it, and the highlight sat on the label — the rail lit up one word wide, which reads as the word
 * being the target rather than the row.
 *
 * What this cannot do is change the text: bold or underlined on hover is a property of the glyphs,
 * and an indication draws behind and around them. Saying that out loud rather than approximating it.
 */
class TackuIndication(
    private val hovered: Color,
    private val pressed: Color,
) : IndicationNodeFactory {
    override fun create(interactionSource: InteractionSource): DelegatableNode =
        Node(interactionSource, hovered, pressed)

    override fun equals(other: Any?): Boolean =
        other is TackuIndication && other.hovered == hovered && other.pressed == pressed

    override fun hashCode(): Int = 31 * hovered.hashCode() + pressed.hashCode()

    private class Node(
        private val interactionSource: InteractionSource,
        private val hovered: Color,
        private val pressed: Color,
    ) : Modifier.Node(),
        DrawModifierNode {
        private var isHovered = false
        private var isPressed = false

        override fun onAttach() {
            coroutineScope.launch {
                interactionSource.interactions.collect { interaction ->
                    when (interaction) {
                        is HoverInteraction.Enter -> isHovered = true
                        is HoverInteraction.Exit -> isHovered = false
                        is PressInteraction.Press -> isPressed = true
                        is PressInteraction.Release, is PressInteraction.Cancel -> isPressed = false
                        else -> Unit
                    }
                    invalidateDraw()
                }
            }
        }

        override fun ContentDrawScope.draw() {
            when {
                isPressed -> drawRect(pressed)
                isHovered -> drawRect(hovered)
            }
            drawContent()
        }
    }
}
