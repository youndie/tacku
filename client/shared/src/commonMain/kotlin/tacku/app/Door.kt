package tacku.app

/**
 * How a person gets in, which is not the same question on every platform.
 *
 * In a browser it is a redirect to the identity provider and back — the product's door. On the
 * desktop there is no door of the platform's own: the instrument shows the server's sign-in form,
 * which a shipped server does not serve. That asymmetry is the decision, not an accident of what was
 * easy: a second way into a product is worth having only where nothing is at stake.
 */
interface Door {
    /**
     * A token, if one can be had without asking anybody anything.
     *
     * On a page this is where a return from the provider is finished — the address bar carries a
     * code, and the code becomes a token. Null means nobody is signed in yet.
     */
    suspend fun resume(): String?

    /**
     * Send the person to sign in.
     *
     * On a page this does not return: the tab navigates away and the application starts again when
     * it comes back.
     */
    suspend fun open()

    /**
     * A fresh token for the same person, without asking them anything.
     *
     * Null means there is nothing to renew with, and the caller then has to send them back to the
     * provider. It exists because a token from a provider lives minutes: a page open longer than
     * that answers 401 to everything and looks broken, while nobody signed out and nothing failed.
     */
    suspend fun renew(): String?

    /**
     * Forget whoever was signed in.
     *
     * Signing out means starting again, and starting again asks [resume] first — so without this the
     * door hands back the token it still remembers and the person never leaves. That is what
     * happened the first time the page was signed out of: the screen blinked and stayed.
     */
    fun close()
}

/**
 * The door this platform has, or none.
 *
 * None is the desktop's answer, and the caller falls back to the server's own form — which is only
 * there in a build that carries the instrument's door.
 */
expect fun platformDoor(baseUrl: String): Door?
