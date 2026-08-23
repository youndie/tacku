package tacku.app

import io.ktor.client.HttpClient
import io.ktor.client.request.forms.submitForm
import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.Parameters
import kotlinx.browser.window
import kotlinx.coroutines.await
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.w3c.dom.get
import org.w3c.dom.set
import kotlin.random.Random

/**
 * The page's door: a redirect to the identity provider and back, with PKCE.
 *
 * PKCE and not a secret, because a page keeps none — every byte it holds is readable by whoever is
 * looking at it. The verifier stays in session storage for the moment the tab is away, and the code
 * that comes back is worthless without it.
 *
 * Nothing here is configured at build time. The address, the client's name and the audience are
 * asked of the server that served this page, which is the same server the tokens will be spent at;
 * a bundle with a provider compiled into it would be a bundle per deployment.
 */
private class RedirectDoor(
    private val baseUrl: String,
) : Door {
    private val http = HttpClient()
    private val json = Json { ignoreUnknownKeys = true }

    override suspend fun resume(): String? {
        // A code in the address bar means the person has just come back from the provider, and it
        // wins over anything already stored. The other order cost a day: a token that had expired
        // stayed in storage, was handed back on every start, and the code that arrived to replace it
        // was never spent — so the provider signed the person in again and again while every
        // request was refused for a token five minutes dead.
        val code = parameter("code")

        if (code == null) {
            // A token already held survives a reload. It does not survive a closed tab, which is the
            // right trade for a product where signing in costs one redirect: a token in local
            // storage outlives the person's intention to be signed in.
            val held = window.sessionStorage[TOKEN]
            if (held != null && !TokenLife.isSpent(held, nowSeconds())) return held

            // Held and spent: renewing is silent when there is a refresh token and null when there
            // is not, and null is what sends the person to the provider. Presenting the dead one
            // instead would buy a refusal the page would then have to interpret.
            if (held != null) return renew()

            return null
        }

        val verifierOrNull = window.sessionStorage[VERIFIER]
        val verifier = verifierOrNull ?: return null
        val state = window.sessionStorage[STATE]

        // The state is checked before the code is spent. Without it a page will happily finish a
        // sign-in somebody else started — which is the whole of the attack, and it costs one
        // comparison to refuse.
        if (state != null && state != parameter("state")) {
            println("tacku: the sign-in that came back is not the one this page started")
            clean()
            return null
        }

        val config = config()
        val discovery = discovery(config.issuer)

        val answer =
            http
                .submitForm(
                    url = discovery.tokenEndpoint,
                    formParameters =
                        Parameters.build {
                            append("grant_type", "authorization_code")
                            append("code", code)
                            append("code_verifier", verifier)
                            append("client_id", config.clientId)
                            append("redirect_uri", redirectUri())
                            // Named again at the exchange, because that is the request that
                            // produces the token; the one above only produces a code.
                            append("resource", config.audience)
                        },
                ).bodyAsText()

        val token = keep(answer)
        clean()
        if (token == null) {
            println("tacku: the provider did not hand over a token: $answer")
            return null
        }

        // The code is spent, so the address bar should stop carrying it: a reload would otherwise
        // try to spend it again and be refused for a reason that reads like a broken sign-in.
        window.history.replaceState(null, "", window.location.pathname)
        return token
    }

    /**
     * Trade the refresh token for a new pair.
     *
     * The resource is named again: an audience is a property of a token, not of a session, and a
     * renewed token that named nothing would be refused by the very server it was renewed for.
     *
     * A refusal here is not reported as a failure — a refresh token expires or is revoked, and both
     * mean "sign in again" rather than "something is broken". The caller reads null as that.
     */
    override suspend fun renew(): String? {
        val refresh = window.sessionStorage[REFRESH] ?: return null
        val config = config()
        val discovery = discovery(config.issuer)

        val answer =
            runCatching {
                http
                    .submitForm(
                        url = discovery.tokenEndpoint,
                        formParameters =
                            Parameters.build {
                                append("grant_type", "refresh_token")
                                append("refresh_token", refresh)
                                append("client_id", config.clientId)
                                append("resource", config.audience)
                            },
                    ).bodyAsText()
            }.getOrNull() ?: return null

        return keep(answer)
    }

    /**
     * Store what the provider handed over, and hand back the access token.
     *
     * The refresh token is stored **every time**, because a provider that rotates them hands back a
     * new one with each renewal and keeps the old one only long enough to notice it being reused.
     */
    private fun keep(answer: String): String? {
        val fields = runCatching { json.parseToJsonElement(answer).jsonObject }.getOrNull() ?: return null
        val token = fields["access_token"]?.jsonPrimitive?.content ?: return null

        window.sessionStorage[TOKEN] = token
        fields["refresh_token"]?.jsonPrimitive?.content?.let { window.sessionStorage[REFRESH] = it }
        return token
    }

    override suspend fun open() {
        val config = config()
        val discovery = discovery(config.issuer)

        val verifier = randomString()
        val state = randomString()
        window.sessionStorage[VERIFIER] = verifier
        window.sessionStorage[STATE] = state

        val challenge = sha256(verifier)
        val url =
            discovery.authorizationEndpoint +
                "?response_type=code" +
                "&client_id=" + encode(config.clientId) +
                "&redirect_uri=" + encode(redirectUri()) +
                "&state=" + encode(state) +
                "&code_challenge=" + encode(challenge) +
                "&code_challenge_method=S256" +
                // `offline_access` is what asks for a refresh token, and without it the provider
                // is right not to issue one: a long-lived credential for somebody who did not ask
                // is risk without benefit. Asking is what turns a five-minute page into a session.
                "&scope=" + encode("openid tasks:read tasks:write offline_access") +
                // RFC 8707: which resource the token is for. Without it a provider that binds
                // audiences hands back a token addressed to everything this client may reach, and
                // this server would rather be named than included.
                "&resource=" + encode(config.audience)

        window.location.href = url
    }

    private suspend fun config(): AuthConfig {
        val body = http.get("$baseUrl/auth/config").bodyAsText()
        val fields = json.parseToJsonElement(body).jsonObject
        return AuthConfig(
            issuer = fields["issuer"]?.jsonPrimitive?.content.orEmpty(),
            clientId = fields["clientId"]?.jsonPrimitive?.content.orEmpty(),
            audience = fields["audience"]?.jsonPrimitive?.content.orEmpty(),
        )
    }

    private suspend fun discovery(issuer: String): Discovery {
        val body = http.get("$issuer/.well-known/openid-configuration").bodyAsText()
        val fields = json.parseToJsonElement(body).jsonObject
        return Discovery(
            authorizationEndpoint = fields["authorization_endpoint"]?.jsonPrimitive?.content.orEmpty(),
            tokenEndpoint = fields["token_endpoint"]?.jsonPrimitive?.content.orEmpty(),
        )
    }

    private fun redirectUri(): String = window.location.origin + window.location.pathname

    private fun parameter(name: String): String? =
        window.location.search
            .removePrefix("?")
            .split("&")
            .firstOrNull { it.startsWith("$name=") }
            ?.substringAfter("=")
            ?.let { decode(it) }

    override fun close() {
        window.sessionStorage.removeItem(TOKEN)
        window.sessionStorage.removeItem(REFRESH)
        clean()
    }

    /** Now, in seconds, which is the unit `exp` is in. */
    private fun nowSeconds(): Long = (jsNow() / 1000).toLong()

    private fun clean() {
        window.sessionStorage.removeItem(VERIFIER)
        window.sessionStorage.removeItem(STATE)
    }

    private data class AuthConfig(
        val issuer: String,
        val clientId: String,
        /** What this server will insist the token is addressed to — so the page asks for it by name. */
        val audience: String,
    )

    private data class Discovery(
        val authorizationEndpoint: String,
        val tokenEndpoint: String,
    )

    private companion object {
        const val TOKEN = "tacku.token"
        const val REFRESH = "tacku.refresh"
        const val VERIFIER = "tacku.verifier"
        const val STATE = "tacku.state"
    }
}

actual fun platformDoor(baseUrl: String): Door? = RedirectDoor(baseUrl)

private fun randomString(): String {
    val alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
    return (1..64).map { alphabet[Random.nextInt(alphabet.length)] }.joinToString("")
}

/** The clock, from the page's own runtime: `exp` is measured against it and not against ours. */
private fun jsNow(): Double = js("Date.now()")

private fun encode(value: String): String = js("encodeURIComponent(value)")

private fun decode(value: String): String = js("decodeURIComponent(value)")

/**
 * The challenge: base64url of the verifier's digest, computed by the browser's own subtle crypto.
 *
 * Borrowed rather than written. A hash implemented here would be a second implementation of
 * something every browser already has, wrong in ways that only appear against a real provider.
 */
private suspend fun sha256(value: String): String = jsSha256(value).await().toString()

private fun jsSha256(value: String): kotlin.js.Promise<kotlin.js.JsString> =
    js(
        """(async () => {
            const data = new TextEncoder().encode(value);
            const digest = await crypto.subtle.digest('SHA-256', data);
            let binary = '';
            new Uint8Array(digest).forEach(b => binary += String.fromCharCode(b));
            return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
        })()""",
    )
