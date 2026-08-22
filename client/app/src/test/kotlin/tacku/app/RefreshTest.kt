package tacku.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import io.github.youndie.kompot.KompotLazyScreen
import io.github.youndie.kompot.KompotRealtimeProvider
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import io.github.youndie.kompot.realtime.KompotRealtimeSource
import io.github.youndie.kompot.realtime.UpdateComponentMessage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.emptyFlow
import kotlin.test.Test

/**
 * A screen inside the live channel shows a tree it is given after the first one.
 *
 * "The card moved but the board did not change" is a sentence about this and nothing else. The data
 * path is provably right: the move performs, the answer is a navigate, the reload comes back with
 * the card in its new column — the probe prints exactly that against a running server. What was left
 * was whether the drawing follows.
 *
 * Two trees and a swap is the whole question, so there is no server here. The channel is silent for
 * the same reason: the wrapper is the only thing being measured.
 */
class RefreshTest {
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `a screen inside the realtime provider shows a new tree`() =
        runComposeUiTest {
            var body by mutableStateOf(BEFORE)

            setContent {
                TackuTheme(TackuDesignSystem()) {
                    CompositionLocalProvider(LocalKompotPageLoader provides transport.pageLoader()) {
                        KompotRealtimeProvider(
                            topic = "self",
                            source = Silent,
                            content = {
                                Box(Modifier.size(400.dp, 200.dp)) {
                                    KompotLazyScreen(
                                        rootComponent = transport.decodeScreen(body),
                                        registry = tackuRegistry(),
                                        formController =
                                            FormController(
                                                schema = FormSchema(formId = "swap", fields = emptyList()),
                                                scope = CoroutineScope(Dispatchers.Unconfined),
                                            ),
                                        actionHandler = { },
                                    )
                                }
                            },
                        )
                    }
                }
            }

            onNode(hasText("before")).assertExists()

            body = AFTER
            waitForIdle()

            onNode(hasText("after")).assertExists()
        }

    /** A channel that never says anything, so the wrapper is the only thing being measured. */
    private object Silent : KompotRealtimeSource {
        override fun subscribe(topic: String): Flow<UpdateComponentMessage> = emptyFlow()
    }

    /**
     * The same, when the thing that changed is inside a `paginated_list`.
     *
     * This is where the board's move ends up. The chain is proved all the way through — the trace of
     * the running application prints intent, perform, answered, follow, open, showing — and a fresh
     * board arrives with the card in its new column. If the list holds the items it was first given,
     * none of that reaches the screen: the tree is new, the list is not.
     *
     * **Ignored, and deliberately kept.** It fails, and what it describes is the toolkit's
     * (kompot#40). Deleting it would delete the reproduction; leaving it red would make the gate
     * mean nothing. Un-ignore it when a release claims to fix this — it is the check, not a
     * formality.
     */
    @OptIn(ExperimentalTestApi::class)
    @Test
    @kotlin.test.Ignore
    fun `a paginated list shows the items of a new tree`() =
        runComposeUiTest {
            var body by mutableStateOf(LIST_BEFORE)

            setContent {
                TackuTheme(TackuDesignSystem()) {
                    CompositionLocalProvider(LocalKompotPageLoader provides transport.pageLoader()) {
                        KompotRealtimeProvider(
                            topic = "self",
                            source = Silent,
                            content = {
                                Box(Modifier.size(400.dp, 300.dp)) {
                                    KompotLazyScreen(
                                        rootComponent = transport.decodeScreen(body),
                                        registry = tackuRegistry(),
                                        formController =
                                            FormController(
                                                schema = FormSchema(formId = "swap", fields = emptyList()),
                                                scope = CoroutineScope(Dispatchers.Unconfined),
                                            ),
                                        actionHandler = { },
                                    )
                                }
                            },
                        )
                    }
                }
            }

            onNode(hasText("first")).assertExists()

            body = LIST_AFTER
            waitForIdle()

            onNode(hasText("second")).assertExists()
        }

    private companion object {
        const val BEFORE =
            """{"type":"column","id":"swap","children":[{"type":"text","id":"swap-line","text":"before","style":"body"}]}"""

        const val AFTER =
            """{"type":"column","id":"swap","children":[{"type":"text","id":"swap-line","text":"after","style":"body"}]}"""

        const val LIST_BEFORE =
            """{"type":"column","id":"swap","children":[{"type":"paginated_list","id":"swap-list","initialItems":[""" +
                """{"type":"text","id":"swap-item","text":"first","style":"body"}],""" +
                """"emptyState":{"type":"text","id":"swap-empty","text":"","style":"body"}}]}"""

        const val LIST_AFTER =
            """{"type":"column","id":"swap","children":[{"type":"paginated_list","id":"swap-list","initialItems":[""" +
                """{"type":"text","id":"swap-item","text":"second","style":"body"}],""" +
                """"emptyState":{"type":"text","id":"swap-empty","text":"","style":"body"}}]}"""
    }
}
