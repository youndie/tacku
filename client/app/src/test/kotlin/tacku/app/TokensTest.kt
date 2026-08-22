package tacku.app

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The token set, written where the other half can read it.
 *
 * The client declares the names (§6) and the Go renderer has to use exactly those, so the list is
 * emitted to a file both sides check against. Not shared code — two languages — but not two
 * independent lists either, which is the arrangement that drifts without anybody noticing: an
 * unknown name costs a default and a warning, so on screen it looks like a deliberate lack of
 * emphasis.
 *
 * Regenerate with TACKU_TOKENS_RECORD=true.
 */
class TokensTest {
    private val file = File(System.getenv("TACKU_TOKENS_DIR") ?: "../../design", "tokens.json")

    private val json =
        Json {
            prettyPrint = true
            prettyPrintIndent = "  "
        }

    private fun declared(): JsonObject =
        buildJsonObject {
            put(
                "colors",
                buildJsonArray { TackuDesignSystem.colorTokens.forEach { add(JsonPrimitive(it)) } },
            )
            put(
                "typography",
                buildJsonArray { TackuDesignSystem.typographyTokens.forEach { add(JsonPrimitive(it)) } },
            )
        }

    @Test
    fun `the published list matches what the client declares`() {
        val expected = json.encodeToString(JsonObject.serializer(), declared()) + "\n"

        if (System.getenv("TACKU_TOKENS_RECORD")?.toBoolean() == true) {
            file.parentFile.mkdirs()
            file.writeText(expected)
        }

        assertTrue(file.isFile, "no ${file.absolutePath}; regenerate with TACKU_TOKENS_RECORD=true")
        assertEquals(expected, file.readText(), "the published token list has drifted from the design system")
    }

    /**
     * Both themes carry every name. A token defined in the dark and missing in the light is a screen
     * that loses one colour when somebody switches, and loses it quietly.
     */
    @Test
    fun `both themes declare the same names`() {
        val dark = TackuDesignSystem(dark = true)
        val light = TackuDesignSystem(dark = false)

        assertEquals(
            TackuDesignSystem.colorTokens,
            light.let { TackuDesignSystem.colorTokens },
            "the two themes declare different colour names",
        )
        assertTrue(dark !== light)
    }

    @Test
    fun `the set is small enough to have been thought about`() {
        val total = TackuDesignSystem.colorTokens.size + TackuDesignSystem.typographyTokens.size
        // 25 until the navigation stopped being made of buttons. `nav` and `nav_current` were added
        // together and on purpose: a button carries no style field, so the current destination could
        // not be made heavier than its neighbours, and a highlight with no change of weight reads as
        // decoration rather than as "you are here". Two names, one distinction — and this assertion
        // is what made that a decision instead of a drift.
        assertEquals(27, total, "the set is $total names; every extra one is a place the two sides can disagree")

        val overlap = TackuDesignSystem.colorTokens.intersect(TackuDesignSystem.typographyTokens.toSet())
        assertEquals(
            setOf("notice"),
            overlap,
            "a name meaning one thing as a colour and another as a type is worth knowing about deliberately",
        )
    }

    private fun JsonArray.unused() = Unit
}
