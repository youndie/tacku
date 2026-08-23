package tacku.app

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import ru.workinprogress.viddik.core.ViddikGlyphCoverage
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * Which characters of this product the harness's font cannot draw.
 *
 * A glyph the font lacks is resolved by the host, and viddik says plainly that this is the one thing
 * it cannot make portable. It arrives as a pixel count on somebody else's machine: a back link's
 * arrow is missing from the bundled Roboto, its width differs, and the button beside it moves two
 * pixels — 0.06% of the screen, found after two days of looking at fonts.
 *
 * So the question is asked here, of the corpus rather than of a picture, and the answer is a
 * character rather than a percentage. The set is named in full: a new uncovered character fails this
 * test with its own codepoint, instead of failing a screenshot with a number.
 */
class ScreenTextCoverageTest {
    /**
     * What we know the bundled font does not draw, and what each costs.
     *
     * `←` (U+2190) is in the product's own typeface — measured in its cmap — so the product draws it
     * correctly and only the picture is affected. That is why it is tolerated rather than removed
     * from the copy: changing what a person sees to suit a screenshot is the wrong way round.
     */
    private val known = mapOf(0x2190 to "the back link's arrow, which IBM Plex has and Roboto does not")

    @Test
    fun `every character the screens draw is either covered or named`() {
        val dir = File(System.getenv("TACKU_SCREEN_DIR") ?: "src/test/screens")
        val files = dir.listFiles { f -> f.name.endsWith(".json") }.orEmpty()
        check(files.isNotEmpty()) { "no screens in ${dir.absolutePath}: this test would pass by reading nothing" }

        val text = files.joinToString(" ") { textOf(Json.parseToJsonElement(it.readText())) }
        check(text.length > 200) { "the corpus yielded ${text.length} characters of text, which is not a product" }

        val missing = ViddikGlyphCoverage.missingGlyphs(text)

        assertEquals(
            known.keys,
            missing,
            "uncovered characters: " +
                missing.joinToString { "U+%04X %s".format(it, known[it] ?: "new — the host will draw it") },
        )
    }

    /** Every string under a `text`, `label`, `title` or `placeholder` key, wherever it sits. */
    private fun textOf(element: JsonElement): String =
        when (element) {
            is JsonObject ->
                element.entries.joinToString(" ") { (key, value) ->
                    if (value is JsonPrimitive && value.isString && key in drawn) value.content else textOf(value)
                }

            is JsonArray -> element.joinToString(" ") { textOf(it) }
            else -> ""
        }

    private val drawn = setOf("text", "label", "title", "placeholder", "helper", "value")
}
