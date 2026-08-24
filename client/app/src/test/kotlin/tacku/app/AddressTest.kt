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

/**
 * Every destination the server trusts this client to resolve, resolved.
 *
 * The server keeps a list of deeplinks it emits without a graph route and asserts that each is on
 * the client's list — but the client's list was a constant nobody read: `app://edit-task/TAC-2`
 * resolved to nothing, and following such a link opened the first screen instead. Two lists agreed
 * with each other while one of them was fiction.
 */
class PrefixedDestinationsTest {
    /**
     * The kind and the address agree.
     *
     * Written after they did not: the kind was `"form"` for every prefixed destination, which was
     * true of the two that existed and false of the third the day it was added. The client asked
     * for an envelope, the server sent a tree, and the application died on
     * `Fields [schema, screen] not found` the first time somebody pressed a card — while a test
     * that compared only addresses stayed green.
     *
     * The rule is the server's own: a form is served under `/forms/`, a screen under `/screens/`.
     */
    @Test
    fun `every prefixed destination declares the kind its address implies`() {
        for ((target, prefix) in Navigator.prefixed) {
            val implied = if (target.path.startsWith("/forms/")) "form" else "screen"
            assertEquals(implied, target.kind, "$prefix leads to ${target.path}")
        }
    }

    @Test
    fun `a task and an edit screen both resolve`() {
        assertEquals("/forms/task/TAC-2", Navigator.resolvePrefixed("app://task/TAC-2")?.path)
        assertEquals("/forms/edit-task/TAC-2", Navigator.resolvePrefixed("app://edit-task/TAC-2")?.path)
        assertEquals("/screens/docs-item/B-171", Navigator.resolvePrefixed("app://docs-item/B-171")?.path)
    }

    @Test
    fun `a prefix with nothing after it names no screen`() {
        assertNull(Navigator.resolvePrefixed("app://task/"))
        assertNull(Navigator.resolvePrefixed("app://edit-task/"))
        assertNull(Navigator.resolvePrefixed("app://docs-item/"))
        assertNull(Navigator.resolvePrefixed("app://board"))
    }
}
