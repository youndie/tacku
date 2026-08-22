package tacku.app

import kotlinx.serialization.SerializationException
import tacku.fields.DateField
import tacku.fields.DateInput
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

/**
 * The two halves of a deployment's own type, and what each of them costs when it is missing.
 *
 * The profile declaring a name makes a validator accept the body. It does not make anything decode
 * it, and the two are easy to confuse because both are called "declaring the type". This holds them
 * apart: the first test is what a client of this deployment sees, the second is what a client
 * released before it sees — and the second is the reason the pair ships by deployment order rather
 * than behind a flag, there being no flag (B-26).
 */
class ExtensionTest {
    private val transport = Transport("http://localhost:0")

    @Test
    fun `this client decodes the field type its own server sends`() {
        val response = transport.decodeForm(FORM_WITH_A_DATE)

        val field = response.schema.fields.single { it.fieldId == "due" }
        assertTrue(field is DateField, "the due date decoded as ${field::class.simpleName}, not as a date")
        assertEquals("2026-08-29", (field as DateField).value)

        val input = childrenOf(response.screen).filterIsInstance<DateInput>().single()
        assertEquals("due", input.fieldId)
        assertEquals("d MMM", input.displayFormat, "the format is the server's, and this is where it arrives")
    }

    /**
     * What a client released before the extension sees, shown rather than described.
     *
     * A form field that is not degradable loses the whole response — not the field, the response —
     * so an older client meets this form as a parse failure and an empty screen. That is the entire
     * argument for §15 ordering, and it is cheap to state and expensive to discover.
     */
    @Test
    fun `a client without the extension loses the whole form`() {
        val older = Transport("http://localhost:0", knowsExtensions = false)

        val failure = assertFailsWith<SerializationException> { older.decodeForm(FORM_WITH_A_DATE) }
        assertTrue(
            failure.message.orEmpty().contains("date_field"),
            "the failure does not name the type that caused it: ${failure.message}",
        )
    }

    private fun childrenOf(component: io.github.youndie.kompot.KompotComponent) =
        (component as io.github.youndie.kompot.standard.ColumnComponent).children

    private companion object {
        val FORM_WITH_A_DATE = """
            {
              "schema": {
                "formId": "new-task",
                "fields": [
                  {"type":"date_field","fieldId":"due","rules":[],"value":"2026-08-29"}
                ]
              },
              "screen": {
                "type":"column","id":"form","children":[
                  {"type":"date_input","id":"field-due","fieldId":"due","label":"Due date",
                   "displayFormat":"d MMM","hint":"Leave it empty if there is no deadline."}
                ]
              }
            }
        """
    }
}
