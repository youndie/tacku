package tacku.app

import io.github.youndie.kompot.realtime.UpdateComponentMessage
import io.github.youndie.kompot.standard.TextComponent
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A frame is a component, and this client decodes it as one.
 *
 * The half worth checking here is not the framing — the conformance kit checks that from a recorded
 * stream now — but that a frame's payload goes through the same vocabulary as a screen. A reader
 * with its own serialisers would accept frames a screen would refuse, and the disagreement would
 * show as a node that draws on one path and not on the other.
 */
class UpdatesTest {
    private val updates = Updates("http://localhost:0") { null }

    @Test
    fun `a frame decodes into the component it carries`() {
        val frame =
            updates.decode(
                """{"componentId":"card-TAC-4","component":{"type":"text","id":"t","text":"moved"}}""",
            )

        assertEquals("card-TAC-4", frame.componentId)
        assertTrue(frame.component is TextComponent, "the frame carried ${frame.component::class.simpleName}")
    }

    /**
     * And a type this deployment added travels in a frame like any other.
     *
     * A channel decoding with a narrower vocabulary than the screens would be the quiet kind of
     * wrong: everything works until the day a pushed card contains a date.
     */
    @Test
    fun `a frame may carry a type this deployment added`() {
        val frame =
            updates.decode(
                """{"componentId":"field-due","component":{"type":"date_input","id":"d","fieldId":"due","label":"Due"}}""",
            )

        assertEquals("field-due", frame.componentId)
        assertEquals("tacku.fields.DateInput", frame.component::class.qualifiedName)
    }

    private fun Updates.decode(body: String): UpdateComponentMessage = decodeFrame(body)
}
