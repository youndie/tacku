package tacku.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotDisplayed
import androidx.compose.ui.test.hasScrollAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * A form taller than the window can be reached.
 *
 * Not a hypothetical: the new-task form is six fields and a submit, and on a window this size the
 * button is below the fold. The screen was drawn with `KompotScreen`, which lays its tree out and
 * stops there, so the bottom of the form did not exist as far as a person was concerned — you could
 * open the form and not create a task. Nothing failed and nothing said so; the screenshots are taken
 * at whatever canvas the shot declares, so they never met a window too small.
 *
 * The assertion is in two halves on purpose. Scrolling to a node that is already visible passes
 * trivially, so the first half is that it is *not* visible to begin with — otherwise the test would
 * go on passing if the layout ever stopped overflowing, and prove nothing about scrolling.
 */
class ScrollTest {
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `the bottom of a long form can be reached`() =
        runComposeUiTest {
            val design = TackuDesignSystem()
            val body = screenOf("new-task")

            setContent {
                TackuTheme(design) {
                    Box(Modifier.size(700.dp, 320.dp)) {
                        Inner(body)
                    }
                }
            }

            // Not displayed, rather than not composed. It used to be the second: every field was
            // an item of the list, and a lazy list does not compose what is below the fold, so the
            // node did not exist at all. Since the form's padding moved inside the scroll the whole
            // body is one item — the list composes it entirely and the bottom line exists at y=695
            // of a 320-point window. Composed and off-screen is still unreachable, which is the
            // property this test is about; where the node is in the tree is not.
            onNode(hasText(BOTTOM_OF_THE_FORM, substring = true)).assertIsNotDisplayed()

            onNode(hasScrollAction()).performScrollToNode(hasText(BOTTOM_OF_THE_FORM, substring = true))
            onNode(hasText(BOTTOM_OF_THE_FORM, substring = true)).assertIsDisplayed()
        }

    private companion object {
        /** The last line of the new-task form, and the one a person has to reach to submit. */
        const val BOTTOM_OF_THE_FORM = "Every action stays in the history"

        /** The line under the comment box, which is the bottom of the task screen. */
        const val BOTTOM_OF_THE_TASK = "Posted as you"

        /** The sixth card of TO DO on the seeded board, well below a 420-point window. */
        const val LAST_CARD_IN_TODO = "Measure where people change status"
    }

    /**
     * A board column is a list of its own, and it has to scroll inside its column.
     *
     * The screen-level scroll cannot help here: the board's root is a row, so the whole board is one
     * item, and a column taller than the window has to move its own cards.
     */
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `a board column scrolls to its last card`() =
        runComposeUiTest {
            setContent {
                val design = TackuDesignSystem()
                TackuTheme(design) {
                    Box(Modifier.size(1200.dp, 420.dp)) {
                        Inner(screenOf("board"))
                    }
                }
            }

            // Counted first, because "the scroll did not reach" and "there is nothing here that
            // scrolls" are different findings and only one of them is ours. It was zero once.
            val scrollables = onAllNodes(hasScrollAction()).fetchSemanticsNodes().size
            assertTrue(scrollables > 0, "no node on this board can be scrolled at all")

            // To a matcher, not to a node: a lazy list has not composed what is below the fold, so
            // asking for the node first is asking for something that does not exist yet.
            onAllNodes(hasScrollAction())[0].performScrollToNode(hasText(LAST_CARD_IN_TODO, substring = true))
            onNode(hasText(LAST_CARD_IN_TODO, substring = true)).assertIsDisplayed()
        }

    /**
     * The task screen, which is the one a person reads rather than scans.
     *
     * A description, a history and a comment box add up past the fold on any window worth using, and
     * for a few hours this screen could not be scrolled at all: it had a row at its root, and only a
     * column root gets the projection that scrolls. The comment box is at the bottom, so "cannot
     * scroll" means "cannot comment".
     */
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `the bottom of a task can be reached`() =
        runComposeUiTest {
            setContent {
                val design = TackuDesignSystem()
                TackuTheme(design) {
                    Box(Modifier.size(700.dp, 300.dp)) {
                        Inner(screenOf("task"))
                    }
                }
            }

            val scrollables = onAllNodes(hasScrollAction()).fetchSemanticsNodes().size
            assertTrue(scrollables > 0, "nothing on the task screen can be scrolled")

            onAllNodes(hasScrollAction())[0].performScrollToNode(hasText(BOTTOM_OF_THE_TASK, substring = true))
            onNode(hasText(BOTTOM_OF_THE_TASK, substring = true)).assertIsDisplayed()
        }
}
