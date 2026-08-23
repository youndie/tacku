package tacku.app

/**
 * The desktop has no door of its own.
 *
 * It is an instrument, pointed at a stand, and it signs in with the form that stand serves. A
 * redirect would need a browser to come back to, and there is none: the window is the application.
 */
actual fun platformDoor(baseUrl: String): Door? = null
