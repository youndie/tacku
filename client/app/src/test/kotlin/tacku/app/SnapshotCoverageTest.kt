package tacku.app

import java.io.File
import javax.imageio.ImageIO
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
            "Diagnostics_A_stripe_as_tall_as_its_card.png",
            "Diagnostics_Does_a_control_take_the_product_accent.png",
            "Diagnostics_Does_a_token_carry_colour.png",
            "States_Empty_column.png",
            "States_Provenance___agent_beside_a_person.png",
            "States_Refused_on_the_merits.png",
            "States_Unknown_component.png",
            "Screens_Board.png",
            "Screens_Catch_up.png",
            "Screens_My_tasks.png",
            "Screens_Sign_in.png",
            "Screens_Docs_backlog.png",
            // `Screens_Task` and `Screens_New_task` are missing on purpose, and this list is where
            // that is said out loud: both carry a back link, the harness's font has no `←`, and the
            // host draws that one glyph — so everything after it sits differently on each machine.
            // Eleven compared screens look exactly like thirteen from the outside. B-51.
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

    /**
     * Nothing in this product is black.
     *
     * The palette has no `#000000` in either theme, so a black pixel is not a dark colour — it is a
     * control that drew text without saying what colour, taking `LocalContentColor`, whose default
     * is black outside a Material `Surface`. That is how the consent checkbox on the new-task form
     * came to say "Let my agent keep this task up to date" at 1.06:1 against the background: present
     * in the tree, correct on the wire, and unreadable.
     *
     * Zero rather than a threshold, because zero is what was measured across all twelve goldens once
     * the content colour was provided.
     */
    @Test
    fun `no screenshot draws in black`() {
        val files = snapshots.listFiles { f -> f.name.endsWith(".png") }.orEmpty()
        assertTrue(files.isNotEmpty(), "nothing to look at")

        for (file in files) {
            val image = ImageIO.read(file)
            var black = 0
            for (y in 0 until image.height) {
                for (x in 0 until image.width) {
                    if (image.getRGB(x, y) and 0xFFFFFF == 0) black++
                }
            }
            assertEquals(
                0,
                black,
                "${file.name} has $black black pixels: a control drew its own text and got the" +
                    " default content colour rather than this product's",
            )
        }
    }
}
