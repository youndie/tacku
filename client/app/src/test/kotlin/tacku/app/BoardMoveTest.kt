package tacku.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotLazyScreen
import io.github.youndie.kompot.KompotRealtimeProvider
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import io.github.youndie.kompot.form.standard.TextValue
import io.github.youndie.kompot.standard.ButtonComponent
import io.github.youndie.kompot.standard.ColumnComponent
import io.github.youndie.kompot.standard.PaginatedListComponent
import io.github.youndie.kompot.standard.RowComponent
import io.github.youndie.kompot.standard.TextComponent
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import java.net.Socket
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * Pressing move on a card, through everything the application puts between the finger and the server.
 *
 * Every piece had been checked on its own and every piece worked: the server moves a task, the
 * transport sends the shape it wants, the click reaches an action handler, the probe performs the
 * same action over the wire. The card still did not move in the running product — which is the
 * situation where the next check has to be the whole chain rather than a better piece.
 *
 * The chain assembled here is the application's: the same registry, the same navigator, the same
 * live channel around the screen. The last of those is what found it. A frame said "replace the node
 * `card-TAC-2`" and carried a component that contained a node called `card-TAC-2`, so applying it put
 * the card inside itself, and applying it again did it again: `StackOverflowError` inside
 * recomposition rather than a wrong picture. The card grew an outer node for the gap between list
 * items and the frame went on naming the inner one.
 *
 * Skipped unless TACKU_TEST_PORT names a stand: a test that passes because there was no server is
 * worse than one that is not there, and this one also consumes what it tests — after it runs, that
 * card has moved — so it wants a stand of its own rather than whatever is on the usual port.
 *
 * **It does not pass today, and it is not yet evidence about the product.** The click becomes an
 * action (`ActionTest` proves that, with a live channel and without), the navigator records no
 * refusal, and the server never sees the request. That points at this harness rather than at the
 * application: an action dispatched into `Dispatchers.Unconfined` under the test clock is not the
 * same thing as one dispatched on a running window. What it did find, and what is fixed, is a frame
 * that nested a card inside itself — `StackOverflowError` in recomposition, which is what the board
 * was actually doing.
 */
class BoardMoveTest {
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `the move button on a card moves the card`() {
        // Opt-in rather than "a port happens to be open": somebody looking at a stand on the usual
        // port should not have the gate move their cards, and the gate should not depend on whether
        // they are.
        if (System.getenv("TACKU_TEST_PORT") == null || !listening()) {
            println("BoardMoveTest: set TACKU_TEST_PORT to a stand of its own; skipped")
            return
        }

        val transport = Transport("http://$HOST:$PORT")
        val opening = runBlocking { signInAndFetchBoard(transport) }

        // Read off the board rather than named here: this runs against a live database, and a card
        // named in advance may already have been moved by an earlier run.
        val offer = firstOffer(opening) ?: error("no card on this board offers a move at all")
        val (task, label) = offer
        val column = columnOf(opening, task) ?: error("$task is on no column of the board")

        runComposeUiTest {
            var screen by mutableStateOf(opening)
            // A real dispatcher, not Unconfined: the chain is a request, an answer and a reload, and
            // `Unconfined` resumes each step inline on whatever thread finished the last one, which
            // under a test clock is not the arrangement the application runs in.
            val scope = CoroutineScope(Dispatchers.Default)
            // Failures are kept rather than dropped. The first version of this test watched only
            // `Screen.Tree`, so when the chain refused, the test saw a board that had not changed
            // and reported "pressing move changed nothing" — the true statement that hides the
            // reason. A test that ignores the error state measures the same thing as a person who
            // does not read the screen.
            val refusals = mutableListOf<String>()
            val navigator =
                Navigator(transport, scope) { state ->
                    when (state) {
                        is Screen.Tree -> screen = state.component
                        is Screen.Failed -> refusals += state.reason
                        else -> Unit
                    }
                }

            // The application reaches this through update_session when somebody signs in; here the
            // session was taken from the transport directly, so the graph is loaded by hand. Without
            // it every deeplink resolves to nothing and the test measures its own setup.
            runBlocking { navigator.loadGraph() }

            val controller =
                FormController(
                    schema = FormSchema(formId = "", fields = emptyList()),
                    scope = scope,
                )
            val handler = navigator.handler(controller, null)
            val updates = Updates("http://$HOST:$PORT") { transport.accessToken }

            setContent {
                val design = remember { TackuDesignSystem() }
                TackuTheme(design) {
                    CompositionLocalProvider(LocalKompotPageLoader provides transport.pageLoader()) {
                        KompotRealtimeProvider(
                            topic = "self",
                            source = updates,
                            content = {
                                Box(Modifier.size(1200.dp, 900.dp)) {
                                    KompotLazyScreen(
                                        rootComponent = screen,
                                        registry = tackuRegistry(),
                                        formController = controller,
                                        actionHandler = handler,
                                    )
                                }
                            },
                        )
                    }
                }
            }

            // The channel replays its backlog the moment it connects — fourteen frames on this seed,
            // each replacing a card — and a click injected into that churn lands on a node that is
            // being swapped underneath it. A person arrives at a settled screen; the test waits for
            // one too, and that wait is part of the fixture rather than of the product.
            Thread.sleep(SETTLE_MILLIS)
            waitForIdle()

            // Six cards offer the same move; the first in tree order is the one whose task was read.
            onAllNodes(hasText(label, substring = true))[0].performClick()

            // Waited for rather than assumed: the move is a request, an answer and a reload, and
            // none of that has happened when the click returns. `waitForIdle` waits for composition,
            // not for a server.
            runCatching { waitUntil(timeoutMillis = 15_000) { columnOf(screen, task) != column } }

            val after = columnOf(screen, task)
            assertTrue(
                refusals.isEmpty(),
                "the chain refused: ${refusals.joinToString("; ")}",
            )
            assertTrue(
                after != null && after != column,
                "$task was in $column before the move and is in $after after it: pressing move changed nothing",
            )
        }
    }

    private fun listening(): Boolean = runCatching { Socket(HOST, PORT).close() }.isSuccess

    private suspend fun signInAndFetchBoard(transport: Transport): KompotComponent {
        val form = transport.form("/forms/sign-in")
        val session =
            transport.submit(
                "/submit/sign-in",
                form.schema.formId,
                mapOf("email" to TextValue(EMAIL), "password" to TextValue(PASSWORD)),
            )
        transport.useSession(
            (session as? io.github.youndie.kompot.auth.UpdateSessionAction)?.accessToken
                ?: error("signing in answered ${session::class.simpleName}"),
        )
        return transport.screen("/screens/board")
    }

    private companion object {
        const val HOST = "localhost"

        /**
         * A stand of its own, and not the one somebody may be looking at.
         *
         * This test moves cards, so it consumes the board it runs against: after a few runs the
         * first column is empty and it starts reporting that no card offers a move — a failure about
         * the fixture wearing out rather than about the product.
         */
        val PORT = System.getenv("TACKU_TEST_PORT")?.toIntOrNull() ?: 8477

        const val EMAIL = "anna@tacku.team"
        const val PASSWORD = "conformance-stand"
        const val MOVE_PREFIX = "Move to"
        const val SETTLE_MILLIS = 3_000L

        /** The first card that offers a move: which task it is, and what its button says. */
        fun firstOffer(root: KompotComponent): Pair<String, String>? {
            var found: Pair<String, String>? = null

            fun walk(node: KompotComponent) {
                if (found != null) return
                val children = childrenOf(node)

                val label =
                    children
                        .filterIsInstance<ButtonComponent>()
                        .firstOrNull { it.text.startsWith(MOVE_PREFIX) }
                        ?.text
                if (label != null) {
                    val task =
                        children
                            .flatMap { childrenOf(it) }
                            .filterIsInstance<TextComponent>()
                            .firstNotNullOfOrNull { TASK_ID.find(it.text)?.value }
                    if (task != null) {
                        found = task to label
                        return
                    }
                }

                children.forEach(::walk)
            }

            walk(root)
            return found
        }

        /**
         * Which column a task sits in, read the way a person reads it: a column's first line is its
         * heading, and a card carries the identifier in its meta line.
         */
        fun columnOf(
            root: KompotComponent,
            task: String,
        ): String? {
            var found: String? = null

            fun walk(
                node: KompotComponent,
                column: String?,
            ) {
                if (found != null) return
                val here =
                    when (node) {
                        is ColumnComponent ->
                            node.children
                                .filterIsInstance<RowComponent>()
                                .firstNotNullOfOrNull { row ->
                                    row.children
                                        .filterIsInstance<TextComponent>()
                                        .firstOrNull()
                                        ?.text
                                } ?: column
                        else -> column
                    }

                if (node is TextComponent && node.text.contains(task)) {
                    found = here
                    return
                }
                childrenOf(node).forEach { walk(it, here) }
            }

            walk(root, null)
            return found
        }

        fun childrenOf(node: KompotComponent): List<KompotComponent> =
            when (node) {
                is ColumnComponent -> node.children
                is RowComponent -> node.children
                is PaginatedListComponent -> node.initialItems
                else -> emptyList()
            }

        val TASK_ID = Regex("TAC-\\d+")
    }
}
