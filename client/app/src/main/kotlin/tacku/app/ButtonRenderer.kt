package tacku.app

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import io.github.youndie.kompot.KompotActionHandler
import io.github.youndie.kompot.KompotComponentRenderer
import io.github.youndie.kompot.KompotModifierNode
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.SizeType
import io.github.youndie.kompot.TypographyToken
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.standard.ButtonComponent

/**
 * The one standard component this client draws itself, and the reason is the design.
 *
 * **Square.** The design forbids rounding, and the vocabulary agrees with it — there is no radius
 * modifier, so a server cannot ask for a corner and a client that rounds is answering a question
 * nobody was allowed to ask. Material's filled `Button` rounds to a pill and does not take that from
 * the theme: `Shapes` has five slots and the button uses `CornerFull`, which is `CircleShape` in the
 * library rather than a value the theme supplies. Setting every shape slot to a zero radius changes
 * nothing, and the screenshot said so.
 *
 * **Clickable where it is painted.** The accent used to be a `background` modifier around a Material
 * button, so the coloured block and the control were two different rectangles: the ripple stopped
 * short of the edge, and the part of the block that looked like a button was not one. Here the fill,
 * the click and the padding are the same box, in that order — padding last, so it insets the label
 * and not the target.
 *
 * **Emphasis is the fill.** The vocabulary has no button variant; a `background` modifier is the
 * only thing that separates "Sign in" from "Sign out", and the design already names two typography
 * tokens for the two. So the fill decides both the block and the label: a button with a background
 * is the primary one, a button without is quiet. That inference is this client's, not the
 * protocol's — see docs/research/questions.md, Q-58.
 */
class ButtonRenderer : KompotComponentRenderer<ButtonComponent> {
    @Composable
    override fun Render(
        component: ButtonComponent,
        actionHandler: KompotActionHandler,
        formController: FormController,
    ) {
        val design = LocalKompotDesignSystem.current
        val fill = component.modifiers.filterIsInstance<KompotModifierNode.Background>().lastOrNull()
        val padding = component.modifiers.filterIsInstance<KompotModifierNode.Padding>().lastOrNull()
        val sizing = component.modifiers.filterIsInstance<KompotModifierNode.Size>().lastOrNull()

        // Anything else the server puts on a button is dropped here rather than by the toolkit, so
        // it is said out loud. A guard on the server side keeps the list to what this draws
        // (TestEveryButtonModifierIsOneTheClientDraws); this line is what happens if it ever fails.
        component.modifiers
            .filterNot {
                it is KompotModifierNode.Background ||
                    it is KompotModifierNode.Padding ||
                    it is KompotModifierNode.Size
            }.forEach {
                System.err.println(
                    "tacku: button \"${component.id}\" carries $it, which this renderer does not draw",
                )
            }

        SquareButton(
            text = component.text,
            fill = fill?.let { design.resolveColor(it.color) },
            padding = padding.toPaddingValues(),
            sizing = sizing.toModifier(),
        ) { actionHandler.handle(component.action) }
    }
}

/**
 * The shape of every button in this product, in one place.
 *
 * Shared with the date extension rather than copied. That renderer drew four Material buttons of its
 * own, so the single screen where this client adds a component of its own was also the last one
 * still showing pills after the standard button was fixed — a rule that lives in one renderer is a
 * rule about one component.
 */
@Composable
fun SquareButton(
    text: String,
    fill: Color?,
    padding: PaddingValues,
    sizing: Modifier = Modifier,
    onClick: () -> Unit,
) {
    val design = LocalKompotDesignSystem.current
    val style = if (fill != null) "button_primary" else "button_quiet"

    Box(
        sizing
            .background(fill ?: Color.Transparent)
            .clickable { onClick() }
            .padding(padding),
    ) {
        Text(text, style = design.resolveTypography(TypographyToken(style)))
    }
}

/**
 * The default is the label's own box and nothing around it.
 *
 * A button with no padding modifier is a deliberate shape on this wire — the server says how much
 * room a control takes — so this does not invent one.
 */
private fun KompotModifierNode.Padding?.toPaddingValues(): PaddingValues {
    if (this == null) return PaddingValues(0.dp)
    val everywhere = all ?: 0
    return PaddingValues(
        start = (start ?: everywhere).dp,
        top = (top ?: everywhere).dp,
        end = (end ?: everywhere).dp,
        bottom = (bottom ?: everywhere).dp,
    )
}

/**
 * A menu item is as wide as the rail, and that is a `size` rather than a guess.
 *
 * `size` carries `width`/`height` as `Fill` or `Wrap` beside the two dp numbers. This renderer drew
 * neither for a while and said so in the log — which is how the navigation came to be a column of
 * short grey tabs, each as wide as its own word.
 */
private fun KompotModifierNode.Size?.toModifier(): Modifier {
    if (this == null) return Modifier

    var modifier: Modifier = Modifier
    when (width) {
        SizeType.Fill -> modifier = modifier.fillMaxWidth()
        SizeType.Wrap -> modifier = modifier.wrapContentWidth()
        null -> widthDp?.let { modifier = modifier.width(it.dp) }
    }
    when (height) {
        SizeType.Fill -> modifier = modifier.fillMaxHeight()
        SizeType.Wrap -> modifier = modifier.wrapContentHeight()
        null -> heightDp?.let { modifier = modifier.height(it.dp) }
    }
    return modifier
}
