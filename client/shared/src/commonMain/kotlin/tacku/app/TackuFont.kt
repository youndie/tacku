package tacku.app

import androidx.compose.ui.text.font.FontFamily

/**
 * The typeface the design was drawn in, which every platform has to fetch its own way.
 *
 * The bytes are the same three files; how a program gets at them is not. A desktop reads them out of
 * its own jar, a page has to ask the server for them over the network — so this is suspending, and
 * the page waits for it before drawing rather than drawing twice.
 *
 * A family that failed to load is a family in whatever the machine has, which is a picture of the
 * machine rather than of the design. It is worth saying out loud when it happens.
 */
expect suspend fun loadTackuFontFamily(): FontFamily
