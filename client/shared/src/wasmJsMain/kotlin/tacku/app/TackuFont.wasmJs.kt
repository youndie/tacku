package tacku.app

import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.platform.Font
import io.ktor.client.HttpClient
import io.ktor.client.request.get
import io.ktor.client.statement.readRawBytes

/**
 * In a page the font arrives over the network, from the server that served the page.
 *
 * The same three files the desktop reads out of its jar. They are fetched before anything is drawn
 * rather than after: a first frame in the machine's own font and a second in the product's is two
 * pictures where there should be one, and the difference between them is exactly the thing this
 * product spent a day discovering it could not see.
 *
 * A file that does not arrive leaves the default family and says so. Silence here would be the
 * page looking almost right on somebody's machine and nowhere else.
 */
actual suspend fun loadTackuFontFamily(): FontFamily {
    val http = HttpClient()
    val faces =
        listOf(
            "Regular" to FontWeight.Normal,
            "Medium" to FontWeight.Medium,
            "SemiBold" to FontWeight.SemiBold,
        )

    val loaded =
        faces.mapNotNull { (name, weight) ->
            // Absolute for the same reason the script tag is: at /task/TAC-2 a relative path asks
            // for /task/fonts/…, and the answer is the page rather than a font — after which the
            // product draws in whatever the machine has, silently.
            val path = "/fonts/IBMPlexSans-$name.ttf"
            runCatching { http.get(path).readRawBytes() }
                .onFailure {
                    println(
                        "tacku: $path did not load, the page will draw in whatever this browser has: ${it.message}",
                    )
                }.getOrNull()
                ?.let { bytes -> Font("IBMPlexSans-$name", bytes, weight) }
        }

    return if (loaded.isEmpty()) FontFamily.Default else FontFamily(loaded)
}
