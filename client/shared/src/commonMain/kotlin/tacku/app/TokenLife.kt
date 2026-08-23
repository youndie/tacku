package tacku.app

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.io.encoding.Base64
import kotlin.io.encoding.ExperimentalEncodingApi

/**
 * Whether a token is worth presenting.
 *
 * Here rather than beside the storage that holds it, because it is a decision about a string and
 * nothing else — and because the storage lives in a source set no test on this machine can reach.
 *
 * A token whose life cannot be read is treated as **live**: refusing one this cannot parse would
 * turn a shape this client does not understand into a sign-in loop, while presenting it costs one
 * refusal that the server explains.
 */
object TokenLife {
    private val json = Json { ignoreUnknownKeys = true }

    /** Seconds of remaining life to insist on, so a token does not expire in flight. */
    private const val MARGIN = 30

    @OptIn(ExperimentalEncodingApi::class)
    fun isSpent(
        token: String,
        nowSeconds: Long,
    ): Boolean {
        val payload = token.split(".").getOrNull(1) ?: return false
        val decoded =
            runCatching {
                Base64.UrlSafe
                    .withPadding(Base64.PaddingOption.ABSENT_OPTIONAL)
                    .decode(payload)
                    .decodeToString()
            }.getOrNull() ?: return false
        val expiry =
            runCatching {
                json
                    .parseToJsonElement(decoded)
                    .jsonObject["exp"]
                    ?.jsonPrimitive
                    ?.content
                    ?.toLong()
            }.getOrNull() ?: return false

        return nowSeconds + MARGIN >= expiry
    }
}
