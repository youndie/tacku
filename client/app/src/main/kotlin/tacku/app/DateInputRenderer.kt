package tacku.app

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.github.youndie.kompot.KompotActionHandler
import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotComponentRenderer
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.TypographyToken
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.standard.TextValue
import tacku.fields.DateInput
import java.time.DayOfWeek
import java.time.LocalDate
import kotlin.reflect.KClass

/**
 * What a date looks like when it is a date rather than a text box with a mask.
 *
 * The point of the whole extension is here: the field offers the days a person actually names —
 * today, tomorrow, the next Friday — instead of asking them to work out the ISO string. That
 * arithmetic in the head is the scenario the design named, and no combination of existing
 * components can express it, because a `text_input` has no idea what its text means.
 *
 * The value on the wire stays a `text_value` holding an ISO date, so the server reads it exactly
 * the way it read the masked text box. The extension changes what a person does, not what travels.
 */
class DateInputRenderer(
    private val today: () -> LocalDate,
) : KompotComponentRenderer<DateInput> {
    @Composable
    override fun Render(
        component: DateInput,
        actionHandler: KompotActionHandler,
        formController: FormController,
    ) {
        val design = LocalKompotDesignSystem.current
        val state =
            formController
                .getFieldFlow<TextValue>(component.fieldId)
                .collectAsState(initial = null)
                .value

        val chosen = state?.value?.text.orEmpty()
        // The bounds live on the field definition and not on the component: the schema half is what
        // a validator reads, the tree half is all a renderer sees. Enforcing them here would mean
        // asking the controller for a definition it does not hand out, so they stay where they are
        // already checked — on the server, which refuses a date outside them.
        val offers = remember(component.fieldId) { offers(today()) }

        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(component.label, style = design.resolveTypography(TypographyToken("label")))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                offers.forEach { offer ->
                    Button(onClick = { formController.onValueChanged(component.fieldId, TextValue(offer.iso)) }) {
                        Text(offer.label)
                    }
                }
            }
            val shown =
                when {
                    chosen.isEmpty() -> component.placeholder
                    else -> chosen
                }
            Text(shown, style = design.resolveTypography(TypographyToken("value")))
            if (component.hint.isNotEmpty()) {
                Text(component.hint, style = design.resolveTypography(TypographyToken("meta")))
            }
            state?.error?.let { Text(it, style = design.resolveTypography(TypographyToken("error"))) }
        }
    }

    private data class Offer(
        val label: String,
        val iso: String,
    )

    /**
     * The days a person names out loud.
     *
     * Labels are English here and nowhere else in this client, which is the one place the rule
     * about the server owning every string does not reach: these are produced by the client from a
     * date it computed, and the server never sees them. Written down rather than left to be noticed
     * — it is the seam the extension opens, and the reason `displayFormat` comes from the server
     * while these do not.
     */
    private fun offers(from: LocalDate): List<Offer> {
        val friday = generateSequence(from.plusDays(1)) { it.plusDays(1) }.first { it.dayOfWeek == DayOfWeek.FRIDAY }
        return listOf(
            Offer("Today", from.toString()),
            Offer("Tomorrow", from.plusDays(1).toString()),
            Offer("Friday", friday.toString()),
            Offer("Next week", from.plusDays(7).toString()),
        )
    }
}

/**
 * What this deployment draws that the toolkit does not.
 *
 * A map rather than a list because that is what the registry takes, and one entry because one type
 * is what was added. It is a function rather than a value so that the clock comes from the caller:
 * a renderer that reads the wall clock cannot be photographed.
 */
fun tackuRenderers(
    today: () -> LocalDate = { LocalDate.now() },
): Map<KClass<out KompotComponent>, KompotComponentRenderer<out KompotComponent>> =
    mapOf(DateInput::class to DateInputRenderer(today))
