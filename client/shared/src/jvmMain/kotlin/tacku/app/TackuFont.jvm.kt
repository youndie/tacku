package tacku.app

import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.platform.Font

/**
 * The typeface the design was drawn in, carried by the product rather than found on the machine.
 *
 * The mockup is set in IBM Plex Sans at three weights, and the client had been drawing in whatever
 * the system happened to offer — so the running product and the design were never the same picture,
 * and neither was a screenshot of it. That is the same failure the goldens had with their font, one
 * layer down: a style with no family is a picture of the machine.
 *
 * Bundled under the SIL Open Font License, which travels beside the files.
 */
private val family: FontFamily =
    FontFamily(
        // A resource path, read out of the jar: a file path works until the application is
        // packaged and then it is a file that is not there — the sort of difference that only shows
        // on somebody else's machine.
        Font("fonts/IBMPlexSans-Regular.ttf", FontWeight.Normal),
        Font("fonts/IBMPlexSans-Medium.ttf", FontWeight.Medium),
        Font("fonts/IBMPlexSans-SemiBold.ttf", FontWeight.SemiBold),
    )

/** On the desktop the files are in the jar, so there is nothing to wait for. */
actual suspend fun loadTackuFontFamily(): FontFamily = family
