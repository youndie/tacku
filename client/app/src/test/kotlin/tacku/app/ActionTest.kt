package tacku.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import io.github.youndie.kompot.KompotAction
import io.github.youndie.kompot.KompotLazyScreen
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlin.test.Test
import kotlin.test.assertNotNull

/**
 * A card opens and a move moves — checked by clicking, which nothing here did until now.
 *
 * Every check this project had looked at a body or at a picture. Both say what is drawn; neither
 * says what happens when somebody presses it, and a screen whose actions are dead renders perfectly
 * and photographs perfectly.
 */
class ActionTest {
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `a card opens and its move button performs`() =
        runComposeUiTest {
            val handled = mutableListOf<KompotAction>()
            val body = screenOf("board")

            setContent {
                val design = TackuDesignSystem()
                TackuTheme(design) {
                    CompositionLocalProvider(LocalKompotPageLoader provides transport.pageLoader()) {
                        Box(Modifier.size(1200.dp, 900.dp)) {
                            KompotLazyScreen(
                                rootComponent = transport.decodeScreen(body),
                                registry = tackuRegistry(),
                                formController =
                                    FormController(
                                        schema = FormSchema(formId = "board", fields = emptyList()),
                                        scope = CoroutineScope(Dispatchers.Unconfined),
                                    ),
                                actionHandler = { handled += it },
                            )
                        }
                    }
                }
            }

            onAllNodes(hasText("Add rate limit to the auth endpoint", substring = true))[0].performClick()
            assertNotNull(
                handled.firstOrNull(),
                "clicking a card's title handled no action: the card cannot be opened",
            )

            handled.clear()
            onAllNodes(hasText("Move to In progress", substring = true))[0].performClick()
            assertNotNull(
                handled.firstOrNull(),
                "clicking a move button handled no action: the card cannot be moved",
            )
        }
}
