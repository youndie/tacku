package tacku.fields

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotModifierNode
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * A box for prose, which the vocabulary has no way to ask for.
 *
 * `text_input` is `fieldId`, `label`, `placeholder`, `mask`, `uppercase`, `secret` — nothing about
 * lines — and a size modifier changes geometry rather than behaviour, so the design's
 * `text_input [size h 96]` is a single-line box 96 dp tall. A description and a comment are the two
 * texts a person writes in a tracker, and a field that hides what you have written is what people
 * trip over on the first task.
 *
 * **This is the deployment's only available shape, not its preferred one.** The protocol solves the
 * same problem for itself with an optional field — §4.5 made a row clickable that way rather than
 * with a modifier, because an unknown field is ignored (§3) and costs nothing. A deployment cannot
 * add a field to a type it does not own: §2.4 declares NAMES. So the cheapest form of extension is
 * the toolkit author's, and a whole new component type is ours. Recorded as Q-40 before it was
 * decided.
 *
 * **Only the degrading hierarchy is touched, and that is the whole design.** The definition stays
 * `text_field` and the value stays `text_value`, so a client that never heard of this type still
 * parses the form (§2.1) — it loses the box, not the response. What it loses is still real: the
 * field stays declared with nothing to fill it, which is the state §9.2 tells servers to avoid, and
 * the server has no way to name a substitute (Q-42).
 */
@Serializable
@SerialName("multiline_input")
data class MultilineInput(
    override val id: String,
    override val modifiers: List<KompotModifierNode> = emptyList(),
    val fieldId: String,
    val label: String,
    val placeholder: String = "",
    val hint: String = "",
    /**
     * How many lines the box shows before it scrolls.
     *
     * Lines rather than dp, and that is the one measurement the protocol cannot express: its only
     * unit of length is dp (§5.3), while the height that matters here is a count of lines of the
     * reader's own font. Held on the component rather than in a modifier for the same reason — a
     * modifier says how big, a component says what it is made of. Q-41.
     */
    val minLines: Int = 4,
) : KompotComponent
