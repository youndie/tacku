package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.runBlocking
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

/**
 * What happens at minute six.
 *
 * A token from the provider lives five minutes — measured against the real one, not assumed — and
 * before this the page simply stopped working when it expired: every request answered 401, nothing
 * said why, and nobody had signed out. The screen looked broken and the session was merely over.
 *
 * Three cases, and the middle one is the reason the retry is bounded. A renewal that hands back a
 * token the server also refuses would otherwise spin, and a loop reads as a slow server rather than
 * as a wrong audience — the failure that actually happens when a client is configured with the
 * wrong resource.
 */
class SessionRenewalTest {
    private val screen = """{"type":"column","id":"c","children":[]}"""

    private fun transportWith(
        answers: List<HttpStatusCode>,
        renewal: suspend () -> String?,
    ): Pair<Transport, MutableList<String?>> {
        val presented = mutableListOf<String?>()
        var attempt = 0
        val engine =
            MockEngine { request ->
                presented += request.headers[HttpHeaders.Authorization]
                val status = answers.getOrElse(attempt) { answers.last() }
                attempt += 1
                respond(
                    content = if (status == HttpStatusCode.OK) screen else """{"error":"unauthenticated"}""",
                    status = status,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        val transport = Transport(baseUrl = "http://a-server", engine = engine)
        transport.useSession("stale")
        transport.renew = renewal
        return transport to presented
    }

    @Test
    fun `a refused request is retried once with the renewed token`() {
        val (transport, presented) =
            transportWith(listOf(HttpStatusCode.Unauthorized, HttpStatusCode.OK)) { "fresh" }
        // The body is a screen, decoded by the transport, so a wrong retry shows up as a decode
        // failure rather than as a silently different picture.

        val root: KompotComponent = runBlocking { transport.screen("/screens/catch-up") }

        assertEquals("c", root.id, "the retry did not bring back the screen the second answer carried")
        assertEquals(listOf<String?>("Bearer stale", "Bearer fresh"), presented.toList())
    }

    @Test
    fun `a renewal the server also refuses is not retried again`() {
        val (transport, presented) =
            transportWith(listOf(HttpStatusCode.Unauthorized)) { "fresh-but-wrong" }

        assertFailsWith<ServerRefused> { runBlocking { transport.screen("/screens/catch-up") } }

        assertEquals(2, presented.size, "the transport went round again instead of giving up")
    }

    @Test
    fun `with nothing to renew with, the refusal is shown rather than swallowed`() {
        val (transport, presented) = transportWith(listOf(HttpStatusCode.Unauthorized)) { null }

        val refused =
            assertFailsWith<ServerRefused> { runBlocking { transport.screen("/screens/catch-up") } }

        assertEquals(1, presented.size, "a second request was made with nothing new to present")
        assertTrue(refused.status.value == 401, "the refusal lost its status on the way out")
    }
}
