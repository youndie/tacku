package tacku.app

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * The address bar and the deeplinks are the same names.
 *
 * Held to a test because the two directions have to compose: a person copies what the first produced
 * and the second has to give back the screen they were on. A mapping that is nearly reversible is a
 * link that nearly works.
 */
class AddressTest {
    @Test
    fun `a deeplink becomes a path and comes back`() {
        listOf("app://board", "app://catch-up", "app://task/TAC-2", "app://edit-task/TAC-11").forEach {
            val path = Address.pathOf(it)
            assertEquals(it, path?.let(Address::deeplinkOf), "$it did not survive the round trip")
        }
    }

    @Test
    fun `the root names no screen`() {
        // Not the default screen: "no path" means "decide for me", and a caller that cannot tell
        // the two apart would overwrite a person's own choice on every start.
        assertNull(Address.deeplinkOf("/"))
        assertNull(Address.deeplinkOf(""))
    }

    @Test
    fun `something that is not ours is not translated`() {
        assertNull(Address.pathOf("https://example.com/board"))
        assertNull(Address.pathOf("app://"))
    }
}
