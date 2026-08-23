package tacku.web

import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.window.ComposeViewport
import kotlinx.browser.document
import tacku.app.App

/**
 * The page.
 *
 * The address is empty on purpose: the server that served this file is the server that answers its
 * requests, so every path is relative to wherever the page came from. That is what keeps a
 * deployment from having to name itself in a build — the same bundle works on a laptop, on a stand
 * and in production, and none of them needs a variable set correctly.
 */
@OptIn(ExperimentalComposeUiApi::class)
fun main() {
    ComposeViewport(document.body!!) {
        App(baseUrl = "")
    }
}
