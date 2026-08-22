package tacku.app

import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The screenshots exist, and there are as many of them as were declared.
 *
 * Acceptance for a screenshot harness is a file count, never a build colour. viddik generates its
 * tests as JUnit 5 factories, and a module still running JUnit 4 builds green with an empty golden
 * directory — the tests do not fail, they do not run. That cost somebody a green build in a
 * neighbouring project, which is why it was written down before this work started rather than after.
 *
 * The second assertion is the one that trap does not cover, and this project earned it the same
 * afternoon: four pictures were recorded, and the first two showed no text at all. A file count
 * proves the harness ran. It says nothing about whether anything was drawn.
 */
class SnapshotCoverageTest {
    private val snapshots = File(System.getenv("TACKU_SNAPSHOTS_DIR") ?: "src/test/snapshots")

    private val expected =
        setOf(
            "Diagnostics_Does_a_control_take_the_product_accent.png",
            "Diagnostics_Does_a_token_carry_colour.png",
            "States_Empty_column.png",
            "States_Provenance___agent_beside_a_person.png",
            "States_Refused_on_the_merits.png",
            "States_Unknown_component.png",
        )

    @Test
    fun `every declared screenshot has a file`() {
        assertTrue(
            snapshots.isDirectory,
            "no ${snapshots.absolutePath}: the harness did not run, which a green build does not say",
        )

        val onDisk =
            snapshots
                .listFiles { f -> f.name.endsWith(".png") }
                .orEmpty()
                .map { it.name }
                .toSet()
        assertEquals(expected, onDisk, "the recorded screenshots are not the declared ones")
    }

    /**
     * A picture with almost nothing in it is what a headless renderer produces when a font is
     * missing, a theme is absent or a colour matches its background — and it is a valid PNG, so
     * everything downstream is happy.
     *
     * The threshold is deliberately crude. It is not measuring quality; it is asking whether
     * anything happened.
     */
    @Test
    fun `no screenshot is a blank rectangle`() {
        val files = snapshots.listFiles { f -> f.name.endsWith(".png") }.orEmpty()
        assertTrue(files.isNotEmpty(), "nothing to look at")

        for (file in files) {
            val image = javax.imageio.ImageIO.read(file)
            val colours = mutableSetOf<Int>()
            for (x in 0 until image.width step 2) {
                for (y in 0 until image.height step 2) {
                    colours.add(image.getRGB(x, y))
                    if (colours.size > 12) break
                }
            }
            assertTrue(
                colours.size > 12,
                "${file.name} holds ${colours.size} distinct colours: a valid image of almost nothing",
            )
        }
    }
}
