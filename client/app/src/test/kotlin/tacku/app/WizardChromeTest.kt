package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.kompotEngineSerializersModule
import io.github.youndie.kompot.kompotJson
import io.github.youndie.kompot.wizard.WizardScreenComponent
import io.github.youndie.kompot.wizard.kompotWizardSerializersModule
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.descriptors.elementNames
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
     * The chrome of a scenario carries no words from the server.
     *
     * Eight elements, and every one of them is either an identifier or a number for the progress
     * indicator. Every other control the server places on a screen carries its own text —
     * `button.text`, `text_input.label`, `select_option.label` — because the server placed it. The
     * chrome is the one thing the client places on its own, and it is the one thing the server
     * cannot name.
     *
     * The count is asserted first and on purpose: without it a renamed class would make the second
     * assertion pass over an empty list and report that nothing resembling a label was found.
     */
    @OptIn(ExperimentalSerializationApi::class)
    @Test
    fun `the published wizard screen has no field for the label of its finish button`() {
        val elements =
            WizardScreenComponent
                .serializer()
                .descriptor
                .elementNames
                .toList()

        assertEquals(
            listOf("id", "modifiers", "formId", "stepId", "stepIndex", "totalSteps", "canGoBack", "content"),
            elements,
            "the shape of wizard_screen moved; B-31 is about what this list does not contain",
        )
        val words = elements.filter { name -> WORDS.any { name.contains(it, ignoreCase = true) } }
        assertTrue(
            words.isEmpty(),
            "wizard_screen now carries $words — the gap B-31 records has been closed upstream, so read it again",
        )
    }

    /**
     * What it costs to send the label anyway, as an extra key.
     *
     * The schema tolerates one — `additionalProperties: true` on every variant — so a validator
     * accepts the body, and that is what makes the route look available. This is the other end of
     * it: the body decodes, nothing fails, and the label is simply not there afterwards. Tolerated
     * and unreadable are not the same answer, and only one of them is visible from the schema.
     *
     * Re-encoding rather than reading a property is the only way to ask the question at all: there
     * is no property to read, which is the point.
     */
    @Test
    fun `an extra label on a wizard screen survives decoding and arrives nowhere`() {
        val decoded = json.decodeFromString<KompotComponent>(STEP_WITH_A_LABEL)

        assertTrue(decoded is WizardScreenComponent, "the step decoded as ${decoded::class.simpleName}")
        val again = json.encodeToString<KompotComponent>(decoded)
        assertTrue(
            !again.contains("submitLabel") && !again.contains("Delete the board"),
            "the extra key came back out, so this deployment could carry it after all: $again",
        )
    }

    private companion object {
        /** Every way the toolkit names words elsewhere, so that a field under any of them counts. */
        val WORDS = listOf("label", "title", "text", "caption")

        val STEP_WITH_A_LABEL = """
            {
              "type": "wizard_screen",
              "id": "step-confirm",
              "formId": "delete-board",
              "stepId": "confirm",
              "stepIndex": 1,
              "totalSteps": 2,
              "canGoBack": true,
              "submitLabel": "Delete the board",
              "content": {"type": "text", "id": "warning", "text": "This cannot be undone."}
            }
        """
    }
}
