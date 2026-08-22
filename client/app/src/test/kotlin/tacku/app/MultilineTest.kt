package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.standard.ColumnComponent
import tacku.fields.MultilineInput
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * The box for prose, and what a client released before it sees instead.
 *
 * The pair is the whole argument of B-29 held as a test. The date extension had to add a field type,
 * and the cost of that was the entire response; this one adds a component and leaves the definition
 * alone, so the cost is one node. Both sentences are cheap to write and expensive to be wrong about,
 * which is why neither is left as prose.
 */
class MultilineTest {
    private val transport = Transport("http://localhost:0")

    @Test
    fun `this client decodes the box its own server sends`() {
        val response = transport.decodeForm(FORM_WITH_A_DESCRIPTION)

        val children = childrenOf(response.screen)
        val box =
            assertNotNull(
                children.filterIsInstance<MultilineInput>().singleOrNull(),
                "the box decoded as ${children.map { it::class.simpleName }}, and none of that is a multiline input",
            )
        assertEquals("description", box.fieldId)
        assertEquals(6, box.minLines, "the height in lines is the server's, and this is where it arrives")
        assertEquals("What does done look like?", box.placeholder)
    }

    /**
     * The degradation sample: what a client released before the extension actually sees.
     *
     * Not a failure this time — the form parses, and that is the point of putting the box in the
     * hierarchy that has a fallback (§2.1). What it loses is still worth naming: the definition
     * stays declared and nothing draws it, which is exactly the state §9.2 tells a server to avoid,
     * and the server has no channel to say "draw a plain text box instead" (Q-42). Ship-by-order
     * (§15) is the only answer left, and B-26 is why there is no other.
     */
    @Test
    fun `a client without the extension keeps the form and loses the box`() {
        val older = Transport("http://localhost:0", knowsExtensions = false)

        val response = older.decodeForm(FORM_WITH_A_DESCRIPTION)

        val declared = response.schema.fields.singleOrNull { it.fieldId == "description" }
        assertNotNull(declared, "the field disappeared from the schema, so this is not the case being shown")

        val children = childrenOf(response.screen)
        assertEquals(2, children.size, "a node the client does not know cost the rest of the form")

        val substitute = children.last()
        assertTrue(
            substitute::class.simpleName?.contains("Unknown") == true,
            "the box decoded as ${substitute::class.simpleName}, not as a placeholder",
        )
        assertTrue(
            substitute !is MultilineInput,
            "the client without the extension decoded the extension anyway",
        )
    }

    private fun childrenOf(component: KompotComponent) = (component as ColumnComponent).children

    private companion object {
        // A form with nothing of ours in its schema half: `text_field` is the toolkit's own type,
        // and that is what makes the response survive the client below.
        val FORM_WITH_A_DESCRIPTION = """
            {
              "schema": {
                "formId": "new-task",
                "fields": [
                  {"type":"text_field","fieldId":"title","rules":[]},
                  {"type":"text_field","fieldId":"description","rules":[]}
                ]
              },
              "screen": {
                "type":"column","id":"form","children":[
                  {"type":"text_input","id":"field-title","fieldId":"title","label":"Title"},
                  {"type":"multiline_input","id":"field-description","fieldId":"description",
                   "label":"Description","placeholder":"What does done look like?","minLines":6}
                ]
              }
            }
        """
    }
}
