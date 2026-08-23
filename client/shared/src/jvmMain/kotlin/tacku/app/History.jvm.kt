package tacku.app

/**
 * A window has no history.
 *
 * The instrument is one screen at a time with no address and no back button, so there is nothing to
 * record and nothing to be asked to return to. Null rather than an empty implementation: the
 * navigator then does not push anything at all, and the difference between the two platforms stays
 * one line rather than a set of no-ops.
 */
actual fun platformHistory(): History? = null
