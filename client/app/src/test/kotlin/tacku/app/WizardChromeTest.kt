package tacku.app

import io.github.youndie.kompot.kompotEngineSerializersModule
import io.github.youndie.kompot.kompotJson
import io.github.youndie.kompot.wizard.WizardScreenComponent
import io.github.youndie.kompot.wizard.kompotWizardSerializersModule
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.json.Json
import kotlinx.serialization.modules.SerializersModule
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * What the toolkit's own type says about the button that finishes a scenario (B-31).
 *
 * The schema half of this question is answered in the Go half, against the committed schema file.
 * This is the other half, and it is not the same statement: a schema file describes what may
 * travel, and a published class describes what can be held once it has. A field missing from both
 * is missing from the contract; a field missing from only one would be a defect of the generator,
 * and that difference is the whole reason the two are checked separately.
 */
class WizardChromeTest {
    private val json: Json =
        kompotJson(
            SerializersModule {
                include(kompotEngineSerializersModule)
                include(kompotWizardSerializersModule)
            },
        )

    /**
     * The published wizard screen names its finish button, and nothing else.
     *
     * Eight of its nine elements are identifiers or numbers for the progress indicator. Every other
     * control the server places carries its own text — `button.text`, `text_input.label`,
     * `select_option.label` — because the server placed it; the chrome is the one thing the client
     * places on its own, and until kompot 0.21 it was the one thing the server could not name.
     *
     * This asserted the absence and was written to fail the day the field appeared. It did. What
     * replaced it keeps the same two-sided shape: the count is checked first, so a renamed class
     * cannot make the second assertion pass over an empty list.
     */
    @OptIn(ExperimentalSerializationApi::class)
    @Test
    fun `the published wizard screen names its finish button and nothing else`() {
        val descriptor = WizardScreenComponent.serializer().descriptor
        val names = (0 until descriptor.elementsCount).map { descriptor.getElementName(it) }

        assertEquals(9, names.size, "the shape of wizard_screen moved again: $names")
        assertTrue(
            names.contains("finishLabel"),
            "wizard_screen carries $names and no finishLabel — the field this product relies on is gone",
        )

        val worded =
            names.filter { name ->
                listOf("label", "title", "text", "caption").any { name.lowercase().contains(it) }
            }
        assertEquals(
            listOf("finishLabel"),
            worded,
            "the chrome carries $worded: a word the server does not fill is a word somebody else chose",
        )
    }
}
