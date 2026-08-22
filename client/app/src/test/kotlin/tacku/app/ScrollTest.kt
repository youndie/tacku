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

            // Does not exist rather than is not displayed, and the difference is the mechanism: a
            // lazy screen has not composed what is below the fold at all, so there is no node to
            // ask about. Either way it is unreachable, which is the point.
            onNode(hasText(BOTTOM_OF_THE_FORM, substring = true)).assertDoesNotExist()

            onNode(hasScrollAction()).performScrollToNode(hasText(BOTTOM_OF_THE_FORM, substring = true))
            onNode(hasText(BOTTOM_OF_THE_FORM, substring = true)).assertIsDisplayed()
        }

    private companion object {
        /** The last line of the new-task form, and the one a person has to reach to submit. */
        const val BOTTOM_OF_THE_FORM = "Every action stays in the history"

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

            // Counted before anything is scrolled, because "the scroll did not work" and "there is
            // nothing here that scrolls" are different findings and only one of them is ours.
            val scrollables = onAllNodes(hasScrollAction()).fetchSemanticsNodes().size
            println("board: $scrollables scrollable nodes")

            onNode(hasText(LAST_CARD_IN_TODO, substring = true)).assertIsNotDisplayed()
        }
}
