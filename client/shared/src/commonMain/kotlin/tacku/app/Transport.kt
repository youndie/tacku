package tacku.app

import io.github.youndie.kompot.KompotAction
import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.auth.kompotAuthSerializersModule
import io.github.youndie.kompot.commands.kompotCommandsSerializersModule
import io.github.youndie.kompot.form.FieldValue
import io.github.youndie.kompot.form.standard.formStandardSerializersModule
import io.github.youndie.kompot.forms.FormPatchRequest
import io.github.youndie.kompot.forms.KompotFormResponse
import io.github.youndie.kompot.kompotEngineSerializersModule
import io.github.youndie.kompot.kompotJson
import io.github.youndie.kompot.navigation.NavigationGraph
import io.github.youndie.kompot.standard.KompotPageLoader
import io.github.youndie.kompot.standard.KompotPageResponse
import io.ktor.client.HttpClient
import io.ktor.client.request.get
import io.ktor.client.request.header
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.client.statement.HttpResponse
import io.ktor.client.statement.bodyAsText
import io.ktor.http.ContentType
import io.ktor.http.HttpStatusCode
import io.ktor.http.contentType
import kotlinx.serialization.PolymorphicSerializer
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.modules.SerializersModule
import tacku.fields.tackuFieldsSerializersModule
import kotlin.uuid.ExperimentalUuidApi
import kotlin.uuid.Uuid

/**
 * Everything this client knows about talking to a server.
 *
 * The serialisers come from the toolkit — [kompotEngineSerializersModule] plus the plug-in modules —
 * rather than from anything written here. A client that declared its own would be a second opinion
 * about the wire, and the point of this one is to be the toolkit's opinion.
 */
class Transport(
    private val baseUrl: String,
    /**
     * Whether this client knows the wire types this deployment added.
     *
     * False is what a client released before them looks like, and it exists so that the cost can be
     * shown rather than asserted: the form does not lose the date, it fails to parse. Nothing in
     * the product sets it — only the test that demonstrates what §15 ordering is protecting.
     */
    private val knowsExtensions: Boolean = true,
) {
    // No engine named, so each platform brings its own off the classpath: CIO on the desktop, the
    // browser's fetch in a page. The alternative — naming one — is the line that would have to be
    // duplicated per target for no gain.
    private val http = HttpClient()

    /**
     * The engine's serialisers **plus the field plug-in's**, and the second half is not optional.
     *
     * Without it the first form fetched fails with "Serializer for subclass 'text_field' is not
     * found" — and fails the parse of the **whole response**, not of one field. That is §2.2 seen
     * rather than read: the form hierarchies have no fallback, so a type the client does not know
     * costs the screen.
     *
     * It is also §1.2 of the research arriving from the other side. form-standard is a module of the
     * toolkit that the toolkit's own spec does not describe, and here it is a module the toolkit's
     * own engine does not register: every consumer wires it in by hand, on both sides of the wire.
     *
     * Auth and commands are here for the same reason and taught a sharper lesson. Missing them, a
     * sign-in decoded as `UnknownAction` and the application would simply have done nothing —
     * §2.1 working exactly as designed: an unknown action degrades and the screen survives. The two
     * failures arrived minutes apart and are worth holding together. The hierarchy that does not
     * degrade fails loudly and immediately; the one that does degrades **silently**, and a silent
     * success that does nothing is the harder of the two to find.
     */
    private val json: Json =
        kompotJson(
            SerializersModule {
                include(kompotEngineSerializersModule)
                include(formStandardSerializersModule)
                include(kompotAuthSerializersModule)
                include(kompotCommandsSerializersModule)
                // The two types this deployment adds. Without this line the client would meet its
                // own server's date and lose the whole form — the hierarchy it belongs to does not
                // degrade, and the profile declaring the name does not make anything decode it.
                if (knowsExtensions) include(tackuFieldsSerializersModule)
            },
        )

    /**
     * The pair a sign-in hands over, replaced whenever `update_session` arrives (§12.4).
     *
     * No `@Volatile`, which it carried while this was a JVM module: the annotation does not exist
     * everywhere this now runs, the one place that writes it is the navigator's own scope, and a
     * page has a single thread regardless.
     */
    var accessToken: String? = null
        private set

    fun useSession(access: String) {
        accessToken = access
    }

    /** Drop the pair. What the door remembers is the door's to drop. */
    fun forgetSession() {
        accessToken = null
    }

    suspend fun screen(path: String): KompotComponent = decodeScreen(get(path))

    suspend fun form(path: String): KompotFormResponse = json.decodeFromString(get(path))

    suspend fun graph(): NavigationGraph = json.decodeFromString(get("/graph"))

    suspend fun page(url: String): KompotPageResponse = json.decodeFromString(get(url))

    /**
     * The loader the list renderer asks for a next page.
     *
     * Required, not optional: the toolkit reads it from a composition local and throws when it is
     * absent, so a screen carrying a list crashes at render rather than degrading. This client had
     * no loader at all until a screenshot of an empty column failed with "LocalKompotPageLoader not
     * provided" — the feed and the board would both have died on first sight. Decoding a body proves
     * the wire; only drawing it proves the screen.
     *
     * The query the caller passes is the reload case: values of the form on the same screen, sent as
     * parameters, under names §8.4 does not fix — this client uses the field identifiers, which is
     * the assumption Q-04 records on the other side of the wire.
     */
    fun pageLoader(): KompotPageLoader =
        object : KompotPageLoader {
            override suspend fun loadPage(
                url: String,
                query: Map<String, String>,
            ): KompotPageResponse {
                val address =
                    if (query.isEmpty()) {
                        url
                    } else {
                        val separator = if (url.contains('?')) "&" else "?"
                        url + separator + query.entries.joinToString("&") { (key, value) -> "$key=$value" }
                    }
                return page(address)
            }
        }

    /**
     * A submit answers an action, and the caller runs it through the same chain as any other
     * intent (§16.4).
     *
     * The idempotency key is generated per attempt and never reused across them: a key kept for the
     * visit rather than the attempt would mean a request corrected after a refusal receives the old
     * refusal for ever.
     */
    @OptIn(ExperimentalUuidApi::class)
    suspend fun submit(
        path: String,
        formId: String,
        values: Map<String, FieldValue>,
    ): KompotAction {
        val body = json.encodeToString(FormPatchRequest(formId = formId, fieldId = "", values = values))
        val response =
            http.post(baseUrl + path) {
                contentType(ContentType.Application.Json)
                header("Idempotency-Key", Uuid.random().toString())
                accessToken?.let { header("Authorization", "Bearer $it") }
                setBody(body)
            }
        return json.decodeFromString(readBody(response, path))
    }

    /** What a `perform` button does: act on one item of a list. */
    @OptIn(ExperimentalUuidApi::class)
    suspend fun perform(
        url: String,
        payload: Map<String, FieldValue>,
    ): KompotAction {
        val body = json.encodeToString(FormPatchRequest(formId = "", fieldId = "", values = payload))
        val response =
            http.post(baseUrl + url) {
                contentType(ContentType.Application.Json)
                header("Idempotency-Key", Uuid.random().toString())
                accessToken?.let { header("Authorization", "Bearer $it") }
                setBody(body)
            }
        return json.decodeFromString(readBody(response, url))
    }

    private suspend fun get(path: String): String {
        val response =
            http.get(baseUrl + path) {
                accessToken?.let { header("Authorization", "Bearer $it") }
            }
        return readBody(response, path)
    }

    /**
     * The error body is a convention of the server rather than part of the protocol (§16.8), so it
     * is read here and turned into something this client can show.
     */
    private suspend fun readBody(
        response: HttpResponse,
        path: String,
    ): String {
        val text = response.bodyAsText()
        if (response.status.isSuccess()) return text

        val message =
            runCatching { json.decodeFromString<JsonObject>(text)["error"]?.toString()?.trim('"') }
                .getOrNull()
                ?: text.take(200)
        throw ServerRefused(response.status, message, path)
    }

    private fun HttpStatusCode.isSuccess() = value in 200..299

    // Decoding without fetching, so that the behaviour of the two hierarchies can be held to a test
    // rather than to a paragraph.
    //
    // The two roots of the vocabulary are plain interfaces, so their serializer is named here rather
    // than inferred. A reified `decodeFromString<KompotComponent>` compiles on both platforms and
    // works on only one: the JVM finds the polymorphic serializer by reflection at runtime, and a
    // page has no reflection to find it with. Named, both platforms decode the same way.
    fun decodeScreen(body: String): KompotComponent =
        json.decodeFromString(PolymorphicSerializer(KompotComponent::class), body)

    fun decodeForm(body: String): KompotFormResponse = json.decodeFromString(body)

    fun decodeAction(body: String): KompotAction =
        json.decodeFromString(PolymorphicSerializer(KompotAction::class), body)
}

class ServerRefused(
    val status: HttpStatusCode,
    val reason: String,
    val path: String,
) : RuntimeException("$status from $path: $reason")
