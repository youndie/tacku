package tacku.app

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * What signing out has to undo.
 *
 * It shipped doing nothing on the half of the product that has a door: signing out resolved to
 * "start again", starting again asks the door whether anybody is already signed in, and the door
 * still remembered the token it had stored. The screen blinked and the same person was still there.
 * Nothing failed, so nothing said so.
 *
 * Both halves are asserted separately because either one alone leaves the person signed in: the
 * transport's token gets the next request answered, and the door's token gets the next start
 * resumed.
 */
class SignOutTest {
    private class RememberingDoor : Door {
        var token: String? = "a-token-from-a-previous-visit"
        var opened = false

        override suspend fun resume() = token

        override suspend fun open() {
            opened = true
        }

        override suspend fun renew(): String? = null

        override fun close() {
            token = null
        }
    }

    @Test
    fun `sign out resolves to forgetting rather than to starting`() {
        val navigator = navigator(RememberingDoor())

        assertEquals(Navigator.Target.SignOut, navigator.resolve(Navigator.SIGN_OUT))
    }

    @Test
    fun `signing out forgets both the transport and the door`() {
        val door = RememberingDoor()
        val transport = Transport(baseUrl = "http://a-server-nobody-calls")
        transport.useSession("a-token-from-a-previous-visit")

        runBlocking { navigator(door, transport).follow(Navigator.SIGN_OUT) }

        assertNull(transport.accessToken, "the transport would answer the next request signed in")
        assertNull(door.token, "the door would resume the next start signed in")
        assertTrue(door.opened, "and nobody was sent to sign in again")
    }

    private fun navigator(
        door: Door,
        transport: Transport = Transport(baseUrl = "http://a-server-nobody-calls"),
    ) = Navigator(transport, CoroutineScope(Dispatchers.Unconfined), door) { }
}
