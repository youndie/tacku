package tacku.app

/**
 * The address bar, as a translation of a deeplink.
 *
 * A page has one thing a window does not: a person can copy where they are, press back, or reload —
 * and until now all three landed on the same first screen, because the address never changed. What
 * follows is the whole of the mapping, and it is deliberately a translation rather than a second
 * routing table: the deeplinks are the product's names for its screens, and a URL that meant
 * something else would be a second opinion about where a person is.
 *
 * `app://board` becomes `/board`, `app://task/TAC-2` becomes `/task/TAC-2`. The scheme goes; nothing
 * else does.
 */
object Address {
    private const val SCHEME = "app://"

    /** The path for a deeplink, or null when it is not one of ours. */
    fun pathOf(deeplink: String): String? =
        if (deeplink.startsWith(SCHEME) && deeplink.length > SCHEME.length) {
            "/" + deeplink.removePrefix(SCHEME)
        } else {
            null
        }

    /**
     * The deeplink for a path, or null for the root.
     *
     * Null rather than the default screen, because "no path" and "a path naming the default screen"
     * are different things to whoever calls: one means "decide for me", the other means "go here".
     */
    fun deeplinkOf(path: String): String? {
        val trimmed = path.trim('/')
        return if (trimmed.isEmpty()) null else SCHEME + trimmed
    }
}
