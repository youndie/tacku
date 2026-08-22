package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotComponentRenderer
import io.github.youndie.kompot.KompotRegistry
import io.github.youndie.kompot.generated.generatedFormsClientRenderers
import io.github.youndie.kompot.kompotCoreRenderers
import io.github.youndie.kompot.kompotStandardRenderers
import tacku.fields.DateInput
import java.time.LocalDate
import kotlin.reflect.KClass

/**
 * What this deployment draws that the toolkit does not.
 *
 * One entry per wire type this build declares in its profile, and the two lists are held equal by a
 * test rather than by hand: a type registered with the serializer and missing here decodes and then
 * draws as a placeholder — the same picture a client that never heard of it sees, produced by our
 * own client, which makes it the failure easiest to mistake for the protocol working as designed.
 *
 * A function rather than a value so that the clock comes from the caller: a renderer that reads the
 * wall clock cannot be photographed.
 */
fun tackuRenderers(
    today: () -> LocalDate = { LocalDate.now() },
): Map<KClass<out KompotComponent>, KompotComponentRenderer<out KompotComponent>> =
    mapOf(
        DateInput::class to DateInputRenderer(today),
    )

/**
 * The whole set, in one place, because a picture of a different set is a picture of nothing.
 *
 * The screenshots used to build their own registry out of the toolkit's three maps and leave this
 * deployment's renderers out of it — so every golden showed the toolkit's button and none showed
 * ours, and a screenshot suite whose subject is the design was photographing the library. The map
 * union keeps the last entry, which is what lets [ButtonRenderer] replace a standard one; that also
 * means the order here is not decoration.
 */
fun tackuRegistry(today: () -> LocalDate = { LocalDate.now() }): KompotRegistry =
    KompotRegistry(
        kompotCoreRenderers + kompotStandardRenderers + generatedFormsClientRenderers + tackuRenderers(today),
    )
