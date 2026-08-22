package tacku.app

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.github.youndie.kompot.KompotActionHandler
import io.github.youndie.kompot.KompotComponentRenderer
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.TypographyToken
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.standard.TextValue
import tacku.fields.MultilineInput

/**
 * A box a person can see what they wrote in.
 *
 * The whole extension is this one argument to the text field: `singleLine = false` and a height
 * measured in lines. A `text_input` with a size modifier would be a single-line box 96 dp tall —
 * geometry changed, behaviour unchanged — which is what the design drew and what B-29 was opened
 * about.
 *
 * The value is a `text_value` and nothing else, so the server reads a description exactly as it read
 * a title. What travels is unchanged; what changes is that the text is visible while it is written.
 */
class MultilineInputRenderer : KompotComponentRenderer<MultilineInput> {
    @Composable
    override fun Render(
        component: MultilineInput,
        actionHandler: KompotActionHandler,
        formController: FormController,
    ) {
        val design = LocalKompotDesignSystem.current
        val state =
            formController
                .getFieldFlow<TextValue>(component.fieldId)
                .collectAsState(initial = null)
                .value

        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(component.label, style = design.resolveTypography(TypographyToken("label")))
            OutlinedTextField(
                value = state?.value?.text.orEmpty(),
                onValueChange = { formController.onValueChanged(component.fieldId, TextValue(it)) },
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text(component.placeholder) },
                // Lines rather than dp, which is the one thing the protocol has no unit for (§5.3,
                // Q-41): the height that matters is a count of lines of whatever font the reader
                // ended up with, and the same number of dp holds a different number of them.
                minLines = component.minLines,
                singleLine = false,
                isError = state?.error != null,
            )
            if (component.hint.isNotEmpty()) {
                Text(component.hint, style = design.resolveTypography(TypographyToken("meta")))
            }
            state?.error?.let { Text(it, style = design.resolveTypography(TypographyToken("error"))) }
        }
    }
}
