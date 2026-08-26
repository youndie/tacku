package tacku.app

import io.github.youndie.kompot.client.tck.ClientCorpusResources
import io.github.youndie.kompot.client.tck.ClientCorpusRunner
import io.github.youndie.kompot.client.tck.KompotFormClient
import io.github.youndie.kompot.form.FieldValue
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormPatch
import io.github.youndie.kompot.form.FormSchema
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.serialization.PolymorphicSerializer
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * The reading side, held against the corpus that ships with the protocol.
 *
 * Nothing here is domain: the cases are written in the toolkit's own vocabulary, so what they check
 * is what §9 says a client does — visibility, rule order, blur, cross-field rules, patches, what
 * leaves in the payload. The corpus cannot see anything about drawing, and that line is worth
 * keeping in mind before a green run is trusted for more than it says.
 *
 * **The adapter calls the application's own code, not a copy of it.** The payload comes from
 * [Navigator.payloadOf], which is what the submit handler runs; the JSON comes from [tackuJson],
 * which is what the transport reads with. Assembled again here, both would be a second answer that
 * agrees with itself — and the question the corpus asks is precisely whether this client's answer
 * is the specification's.
 */
class ClientCorpusTest {
    @Test
    fun `the client corpus runs against this client`() {
        val cases = ClientCorpusResources.cases()

        // Counted, because a corpus that found no case would pass in silence — the same failure the
        // server-side gate exists to prevent.
        assertTrue(cases.isNotEmpty(), "the corpus handed over no cases at all")

        val report = ClientCorpusRunner(cases) { TackuFormClient() }.run()
        println("client corpus: ${report.casesRun} cases, ${report.findings.size} findings")
        report.findings.forEach { println("  $it") }

        assertTrue(report.isClean, report.toString())
    }
}

/**
 * This client, as the corpus asks to see it: seven operations and nothing else.
 *
 * The engine underneath is the toolkit's [FormController], which is what this deployment uses —
 * running the corpus against something else would be checking a client nobody ships.
 */
private class TackuFormClient : KompotFormClient {
    private val json = tackuJson()
    private val scope = CoroutineScope(Dispatchers.Unconfined)

    private lateinit var controller: FormController
    private var sent: Map<String, FieldValue>? = null

    override fun load(form: JsonObject) {
        controller = FormController(schema = json.decodeFromJsonElement(FormSchema.serializer(), form), scope = scope)
        sent = null
    }

    // Named rather than reflected, and this is the rule this repository keeps a guard for: the
    // hierarchy is open, `FieldValue.serializer()` does not exist, and a client that reached for it
    // would compile on the desktop and die in a browser (Q-67).
    override fun set(
        fieldId: String,
        value: JsonObject,
    ) = controller.onValueChanged(fieldId, json.decodeFromJsonElement(PolymorphicSerializer(FieldValue::class), value))

    override fun blur(fieldId: String) = controller.onFieldBlurred(fieldId)

    override fun applyPatch(patch: JsonObject) =
        controller.applyPatch(json.decodeFromJsonElement(FormPatch.serializer(), patch))

    // One call, deliberately. Written as the application's steps repeated here, three breakages of
    // the real behaviour left this corpus green — it was checking its own copy of the answer.
    override fun submit() {
        sent = Navigator.sendable(controller)
    }

    override fun visibleFields(): List<String> =
        controller.fieldsState.value.keys
            .filter { controller.isFieldVisible(it) }

    override fun errors(): Map<String, String> =
        controller.fieldsState.value
            .mapNotNull { (id, state) -> state.error?.let { id to it } }
            .toMap()

    override fun payload(): JsonObject? =
        sent?.let { values ->
            JsonObject(
                values.mapValues { (_, value) ->
                    json.encodeToJsonElement(PolymorphicSerializer(FieldValue::class), value)
                },
            )
        }
}

/**
 * The toolkit clears an error when the field carrying it goes away.
 *
 * A test against the dependency and not against this code, which is the honest shape for it. This
 * client refuses a submit when any field has an error; whether an error survives its field being
 * hidden is the toolkit's answer, and the whole behaviour rests on it — a refusal caused by a field
 * nobody can see is a refusal for a reason nobody can act on.
 *
 * It was written the other way round first: a filter here skipped invisible fields, and breaking
 * that filter changed nothing anywhere, because the state it guarded against cannot occur. The
 * filter is gone; the assumption it stood on is checked here, where it can go red if the toolkit
 * ever changes its mind.
 *
 * The corpus does not carry this case — it checks that a hidden field is not validated and that it
 * leaves the payload (§9.4), not what happens to an error already raised on one.
 */
class HiddenErrorTest {
    @Test
    fun `an error on a field that has been hidden does not block the submit`() {
        val schema =
            tackuJson().decodeFromString(
                FormSchema.serializer(),
                """
                {
                  "formId": "f",
                  "fields": [
                    {"type": "checkbox_field", "fieldId": "byPost", "rules": []},
                    {"type": "text_field", "fieldId": "address",
                     "visibleIf": {"type": "equals", "fieldId": "byPost",
                                   "expectedValue": {"type": "boolean_value", "value": true}},
                     "rules": [{"type": "required", "errorMessage": "An address is required"}]}
                  ]
                }
                """.trimIndent(),
            )

        val controller = FormController(schema = schema, scope = CoroutineScope(Dispatchers.Unconfined))
        val ticked = JsonObject(mapOf("type" to JsonPrimitive("boolean_value"), "value" to JsonPrimitive(true)))
        val unticked = JsonObject(mapOf("type" to JsonPrimitive("boolean_value"), "value" to JsonPrimitive(false)))
        val serializer = PolymorphicSerializer(FieldValue::class)

        // Shown, left empty, and told so.
        controller.onValueChanged("byPost", tackuJson().decodeFromJsonElement(serializer, ticked))
        controller.onFieldBlurred("address")
        assertTrue(Navigator.refused(controller), "an empty required field that is shown has to stop the submit")

        // Hidden again, with the error still on it.
        controller.onValueChanged("byPost", tackuJson().decodeFromJsonElement(serializer, unticked))
        assertFalse(
            Navigator.refused(controller),
            "the submit is stopped by an error on a field nobody can see, so nothing on the screen says why",
        )
    }
}
