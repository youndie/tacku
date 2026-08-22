package tacku.app

import kotlinx.serialization.SerializationException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

/**
 * The two hierarchies behave in opposite ways, and this client has now met both.
 *
 * Held as a test rather than as a paragraph because the difference decides what a mistake costs: one
 * loses a node, the other loses the screen — and the cheap one fails silently, which makes it the
 * harder of the two to notice.
 */
class DegradationTest {
    private val transport = Transport("http://localhost:0")

    @Test
    fun `an unknown component degrades to a placeholder and the screen survives`() {
        val tree =
            """
            {"type":"column","id":"root","children":[
              {"type":"text","id":"a","text":"before"},
              {"type":"timeline","id":"b","events":[]},
              {"type":"text","id":"c","text":"after"}
            ]}
            """.trimIndent()

        val decoded = transport.decodeScreen(tree)
        val children = childrenOf(decoded)

        assertEquals(3, children.size, "a node the client does not know cost the whole tree")
        assertTrue(
            children[1]::class.simpleName?.contains("Unknown") == true,
            "an unknown component decoded as ${children[1]::class.simpleName}, not as a placeholder",
        )
    }

    /**
     * And the same mistake one level down inside a form: no fallback, so the response is lost
     * entirely rather than one field of it (§2.2).
     *
     * This is the most dangerous corner of the protocol, and it is dangerous in the direction that
     * shows: it fails immediately and by name.
     */
    @Test
    fun `an unknown field type loses the whole form`() {
        // The example used to be `date_field`, picked because it was the most plausible type the
        // vocabulary was missing — and then this deployment added it, and the test passed by
        // decoding rather than by failing. Third fixture in this repository to be caught by the
        // same thing on the same day, which is what makes it worth a sentence: a negative example
        // chosen for how real it looks is one somebody eventually makes real.
        val form =
            """
            {"schema":{"formId":"f","fields":[{"type":"colour_field","fieldId":"due","rules":[]}]},
             "screen":{"type":"text","id":"a","text":"hello"}}
            """.trimIndent()

        val failure = assertFailsWith<SerializationException> { transport.decodeForm(form) }
        assertTrue(
            failure.message.orEmpty().contains("colour_field"),
            "the refusal reads ${failure.message}; it must name the type nobody declared",
        )
    }

    /**
     * An action nobody declared degrades too — and that is the quiet one. A button carrying it does
     * nothing at all, with no error anywhere, which is how a sign-in that decoded as UnknownAction
     * looked like a working screen that simply refused to proceed.
     */
    @Test
    fun `an unknown action degrades and does nothing`() {
        val decoded = transport.decodeAction("""{"type":"open_the_pod_bay_doors"}""")
        assertTrue(
            decoded::class.simpleName?.contains("Unknown") == true,
            "an unknown action decoded as ${decoded::class.simpleName}",
        )
    }

    private fun childrenOf(component: io.github.youndie.kompot.KompotComponent) =
        (component as io.github.youndie.kompot.standard.ColumnComponent).children
}
