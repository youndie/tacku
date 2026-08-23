package tacku.app

import kotlinx.browser.window

/**
 * The browser's own history.
 *
 * `pushState` rather than the hash: the address then looks like an address, and it is the same path
 * a person can send to somebody else. That costs one thing on the other side — the server has to
 * answer those paths with the page — and the server does, for exactly the paths this product has
 * screens at, so a typo is still a mistake and still says so.
 */
private class BrowserHistory : History {
    override fun current(): String? = window.location.pathname.takeIf { it.trim('/').isNotEmpty() }

    override fun push(path: String) {
        // Nothing when it is already where we are: a screen that re-renders would otherwise fill
        // the history with copies of itself, and back would then take several presses to move.
        if (window.location.pathname == path) return
        window.history.pushState(null, "", path)
    }

    override fun onPop(listener: (String?) -> Unit) {
        window.addEventListener("popstate", {
            listener(current())
        })
    }
}

actual fun platformHistory(): History? = BrowserHistory()
