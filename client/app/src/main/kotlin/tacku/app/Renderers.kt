package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotComponentRenderer
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
