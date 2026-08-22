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
import io.github.youndie.kompot.KompotRealtimeProvider
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import io.github.youndie.kompot.form.standard.TextValue
import io.github.youndie.kompot.realtime.KompotRealtimeSource
import io.github.youndie.kompot.realtime.UpdateComponentMessage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.runBlocking
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

    /**
     * The same click, with the live channel around the screen — which is how the application draws.
     *
     * The difference between "our navigator did not send it" and "the press never became an action"
     * is the difference between a defect here and a defect in the toolkit, and only this says which.
     * The channel needs no server for the question: the source below never yields a frame, so the
     * only thing under test is the wrapper.
     */
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `a click still becomes an action inside the realtime provider`() =
        runComposeUiTest {
            val handled = mutableListOf<KompotAction>()
            val body = screenOf("board")

            setContent {
                val design = TackuDesignSystem()
                TackuTheme(design) {
                    CompositionLocalProvider(LocalKompotPageLoader provides transport.pageLoader()) {
                        KompotRealtimeProvider(
                            topic = "self",
                            source = Silent,
                            content = {
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
                            },
                        )
                    }
                }
            }

            onAllNodes(hasText("Move to In progress", substring = true))[0].performClick()
            assertNotNull(
                handled.firstOrNull(),
                "inside the realtime provider a click on a move button handled no action at all",
            )
        }

    /**
     * The same again, with a channel that actually delivers — and this is the one that fails.
     *
     * A frame replaces a node by identifier, and the provider applies what it holds every time the
     * screen draws. Pressing a card makes the screen draw. So the node under the finger is rebuilt
     * between the press and the release, and the press never becomes anything: the board's move
     * button does nothing, on every card the channel has ever mentioned.
     *
     * Needs a server, because the point is a channel with something in it; skipped without one.
     */
    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `a click becomes an action while a live channel is delivering`() =
        runComposeUiTest {
            // Opt-in, and never the port somebody may be looking at: this one needs a channel with
            // something in it, so it depends on a live server *and* on what is in its journal.
            val port = System.getenv("TACKU_TEST_PORT")?.toIntOrNull()
            if (port == null || runCatching { java.net.Socket("localhost", port).close() }.isFailure) {
                println("ActionTest: set TACKU_TEST_PORT to a stand of its own; skipped")
                return@runComposeUiTest
            }

            val live = Transport("http://localhost:$port")
            val handled = mutableListOf<KompotAction>()
            val board =
                runBlocking {
                    val form = live.form("/forms/sign-in")
                    val session =
                        live.submit(
                            "/submit/sign-in",
                            form.schema.formId,
                            mapOf(
                                "email" to TextValue("anna@tacku.team"),
                                "password" to TextValue("conformance-stand"),
                            ),
                        )
                    live.useSession(
                        (session as? io.github.youndie.kompot.auth.UpdateSessionAction)?.accessToken
                            ?: error("signing in answered ${session::class.simpleName}"),
                    )
                    live.screen("/screens/board")
                }

            setContent {
                val design = TackuDesignSystem()
                TackuTheme(design) {
                    CompositionLocalProvider(LocalKompotPageLoader provides live.pageLoader()) {
                        KompotRealtimeProvider(
                            topic = "self",
                            source = Updates("http://localhost:$port") { live.accessToken },
                            content = {
                                Box(Modifier.size(1200.dp, 900.dp)) {
                                    KompotLazyScreen(
                                        rootComponent = board,
                                        registry = tackuRegistry(),
                                        formController =
                                            FormController(
                                                schema = FormSchema(formId = "board", fields = emptyList()),
                                                scope = CoroutineScope(Dispatchers.Unconfined),
                                            ),
                                        actionHandler = { handled += it },
                                    )
                                }
                            },
                        )
                    }
                }
            }

            // Long enough for the channel's backlog to arrive and be applied.
            Thread.sleep(3_000)
            waitForIdle()

            onAllNodes(hasText("Move to", substring = true))[0].performClick()
            assertNotNull(
                handled.firstOrNull(),
                "with a channel delivering, a click on a move button handled no action: the node is " +
                    "rebuilt from a stored frame every time the screen draws, and the press is lost",
            )
        }

    /** A channel that never says anything, so the wrapper is the only thing being measured. */
    private object Silent : KompotRealtimeSource {
        override fun subscribe(topic: String): Flow<UpdateComponentMessage> = emptyFlow()
    }
}
