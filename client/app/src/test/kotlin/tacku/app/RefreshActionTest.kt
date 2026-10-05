package tacku.app

import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import io.github.youndie.kompot.standard.RefreshAction
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.headersOf
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.runBlocking
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * `refresh` asks again for the screen on display and changes nothing else.
 *
 * The server answers it to the two buttons whose screen is the one to show next — a card's move on
 * the board, "mark all as seen" on the feed — where it used to answer a navigate to that screen's own
 * deeplink (§16.4). The navigate had a second effect a refresh must not: it was a move, and a move is
 * recorded in the history. So both halves are counted here: the screen was asked for twice, and the
 * history heard about it once.
 *
 * And a client that did not know the word did nothing at all with it (§2.1) — the card moved on the
 * server and the board stood still. That is the failure the first assertion is about.
 */
class RefreshActionTest {
    private class CountingHistory : History {
        val pushed = mutableListOf<String>()

        override fun current(): String? = null

        override fun push(path: String) {
            pushed += path
        }

        override fun onPop(listener: (String?) -> Unit) = Unit
    }

    @Test
    fun `refresh asks again for the screen on display without moving`() {
        val asked = mutableListOf<String>()
        val engine =
            MockEngine { request ->
                asked += request.url.encodedPath
                respond(
                    content = """{"type":"text","id":"item","text":"A backlog item"}""",
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        val transport = Transport(baseUrl = "http://a-server", engine = engine)
        val history = CountingHistory()
        val shown = mutableListOf<Screen>()

        runBlocking {
            val work = SupervisorJob()
            val navigator =
                Navigator(transport, CoroutineScope(coroutineContext + work), history = history) { shown += it }

            navigator.follow(Navigator.DOCS_ITEM_PREFIX + "B-01")
            navigator
                .handler(
                    FormController(
                        schema = FormSchema(formId = "", fields = emptyList()),
                        scope = CoroutineScope(Dispatchers.Unconfined),
                    ),
                    null,
                ).handle(RefreshAction)

            work.children.forEach { it.join() }
            work.complete()
        }

        assertEquals(
            listOf("/screens/docs-item/B-01", "/screens/docs-item/B-01"),
            asked,
            "refresh did not ask again for the screen on display",
        )
        assertEquals(2, shown.filterIsInstance<Screen.Tree>().size, "the answer to refresh was not shown")
        assertEquals(listOf("/docs-item/B-01"), history.pushed, "refresh was recorded as a move")
    }
}
