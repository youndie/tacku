package tacku.app

/**
 * Where the person is, as the platform records it.
 *
 * A page has a history and a window does not, which is why this is a seam rather than a feature: on
 * the desktop there is nothing to record into and nothing to come back from, and the instrument is
 * unaffected by every line of this.
 *
 * What it buys in a browser is the three things a person expects of a page and did not get: the
 * address names the screen they are on, back returns to the previous one, and a reload stays put.
 */
interface History {
    /** The path the page was opened at, or null when it is the root. */
    fun current(): String?

    /** Record a move. Called after a screen is shown, so what is recorded is what is on the screen. */
    fun push(path: String)

    /**
     * Hear about the back button.
     *
     * The listener is given the path the browser moved to, or null for the root. Called once, at
     * start-up: a second listener would answer the same event twice and send the person two screens
     * back.
     */
    fun onPop(listener: (String?) -> Unit)
}

/** The history this platform keeps, or none. */
expect fun platformHistory(): History?
