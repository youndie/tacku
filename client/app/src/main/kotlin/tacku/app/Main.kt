package tacku.app

import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState

fun main() =
    application {
        // The desktop knows how it is configured; the shared half does not and must not.
        Navigator.traceEnabled = System.getenv("TACKU_TRACE") != null

        val state = rememberWindowState(width = 1440.dp, height = 900.dp)
        Window(onCloseRequest = ::exitApplication, title = "tacku", state = state) {
            App(baseUrl = System.getenv("TACKU_URL") ?: "http://localhost:8477")
        }
    }
