package tacku.tck

import io.github.youndie.kompot.tck.RemoteTckTransport
import io.github.youndie.kompot.tck.TckConfig
import io.github.youndie.kompot.tck.TckRunner
import io.ktor.client.HttpClient
import io.ktor.client.engine.cio.CIO
import io.ktor.client.request.header
import io.ktor.client.request.post
import io.ktor.client.request.prepareGet
import io.ktor.client.request.setBody
import io.ktor.client.statement.bodyAsChannel
import io.ktor.client.statement.bodyAsText
import io.ktor.http.ContentType
import io.ktor.http.contentType
import io.ktor.utils.io.readUTF8Line
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject
import java.io.File

/**
 * Walks a running tacku with the conformance kit and judges the report by [TckGate].
 *
 * The spec comes off disk rather than off the classpath: the Go half generates the OpenAPI
 * description and cannot publish a jar, so the two halves meet in a directory of JSON.
 */
fun main() {
    val target = System.getProperty("tacku.tck.target") ?: error("no tacku.tck.target")
    val specDir = File(System.getProperty("tacku.tck.spec") ?: error("no tacku.tck.spec"))

    val spec = readSpec(specDir)

    // No token held here any more, and that is the point of this change.
    //
    // The kit signs in for itself through `loginPath`, which is how it is meant to work: it holds
    // the credential and decides which requests carry it, so the probes `auth-required` makes
    // deliberately without one stay without one. The harness used to inject a header on every
    // request, which covered seven checks and made the eighth report five findings that were the
    // harness lying rather than the server misbehaving.
    val client = HttpClient(CIO)
    val transport = RemoteTckTransport(target, client)

    val report =
        runBlocking {
            TckRunner(
                transport,
                TckConfig(
                    schemas = spec.schemas,
                    openApi = spec.openApi,
                    // No login path: this build has no form to log in with, so the walk stays
                    // anonymous. That is a real limitation and not a configuration choice — it is
                    // why several checks below will have nothing to look at.
                    loginPath = "/submit/sign-in",
                    loginValues =
                        mapOf(
                            "email" to textValue("anna@tacku.team"),
                            "password" to textValue("conformance-stand"),
                        ),
                    // The idempotency check performs a real operation — there is no way to reach
                    // 400 and 409 otherwise, the handler refusing on the merits long before it
                    // looks at the header — so it only runs when a payload is given. This walk is
                    // against a throwaway database, which is where the kit's own README says to
                    // run it.
                    // The two endpoints addressed by naming a thing. Until 0.17 there was nowhere
                    // to put this: the walk skipped them, the report said which, and Q-23 recorded
                    // that the gap was in the kit rather than in this server. The screen of one
                    // task is the most complicated tree this product emits and no check had ever
                    // looked at it.
                    // The live channel, recorded from the running server rather than written here.
                    //
                    // A recording composed by hand would check this harness: it would say the frames
                    // this file can produce satisfy the profile, which nobody doubted. What the walk
                    // is being asked is whether the SERVER's frames do, so the stream is captured
                    // from the endpoint, over the wire, after making something happen on it.
                    recordedUpdateStreams = mapOf("/updates" to recordUpdates(target, client)),
                    pathParameters =
                        mapOf(
                            "/forms/task/{task}" to mapOf("task" to "TAC-1"),
                            "/submit/task-view/{task}" to mapOf("task" to "TAC-1"),
                        ),
                    // One body per submit endpoint the walk can reach. Until 0.15 a missing body
                    // was a silent skip: the report said "9 of 10 endpoints" and left the reader to
                    // guess which, so four of these five were being passed over without anybody
                    // knowing. Two remain unreachable and are recorded as Q-23 rather than papered
                    // over — the walk has no way to supply anything outside the body.
                    submitPayloads =
                        mapOf(
                            "/submit/new-task" to newTaskPayload(),
                            "/submit/new-board" to
                                submission(
                                    "new-board",
                                    // "name", because that is what the form declares. Sending
                                    // "title" produced a 422 and a finding that read like a defect
                                    // in idempotency — the first attempt was refused on its merits,
                                    // so nothing was recorded and the second was refused again.
                                    "name" to textValue("Filed by a conformance walk"),
                                ),
                            // Text on both, and correctly so: this is a `perform`, so the values
                            // are ones the server wrote into the action itself, not ones a person
                            // chose from a control. Sending an entity here refused the first
                            // attempt on its merits, and the retry then read as an idempotency
                            // defect — a finding describing the harness rather than the server, for
                            // the second time in this file.
                            "/submit/move" to
                                submission(
                                    "task_move",
                                    "task" to textValue("TAC-1"),
                                    "status" to textValue("in_progress"),
                                ),
                            // Nothing to fill in: marking everything seen carries no values, and a
                            // submit still has to arrive as a submission.
                            "/submit/seen" to submission("catch_up_seen"),
                            // A checkbox per task and one selector. The identifier is hyphenated
                            // where its neighbours are not, which the form itself explains.
                            "/submit/bulk-move" to
                                submission(
                                    "bulk-move",
                                    "status" to entityValue("in_progress"),
                                    "task-TAC-1" to booleanValue(true),
                                ),
                            // Now reachable, so it needs a body like any other submit. The form is
                            // named after the task it belongs to, which is what carries the subject
                            // the envelope cannot.
                            "/submit/task-view/{task}" to
                                submission(
                                    "task-view/TAC-1",
                                    "status" to entityValue("in_review"),
                                ),
                        ),
                ),
            ).run()
        }

    println(TckGate.describe(report, TckGate.judge(report, spec.openApi)))

    val verdict = TckGate.judge(report, spec.openApi)
    if (!verdict.passed) {
        System.exit(1)
    }
}

private class Spec(
    val schemas: Map<String, JsonObject>,
    val openApi: JsonObject,
)

/**
 * Reads the profile first and then exactly the modules it names, which is what the kit's own reader
 * does: a stray file in the directory is then not part of the contract.
 */
private fun readSpec(dir: File): Spec {
    val json = Json { ignoreUnknownKeys = true }

    fun read(name: String) = json.parseToJsonElement(File(dir, name).readText()).jsonObject

    val profile = read("kompot.profile.schema.json")
    val modules =
        (profile["x-kompot-modules"] as kotlinx.serialization.json.JsonArray)
            .map { (it as kotlinx.serialization.json.JsonPrimitive).content }

    val schemas =
        modules.associate { module -> "$module.schema.json" to read("$module.schema.json") } +
            ("kompot.profile.schema.json" to profile)

    return Spec(schemas, read("kompot.openapi.json"))
}

/**
 * A body the submit endpoint accepts.
 *
 * The shape is `FormPatchRequest`: on a submit the `fieldId` is not significant — it names the field
 * that changed, and on a submit nothing did.
 */
private fun newTaskPayload() =
    submission(
        "new-task",
        "title" to textValue("Filed by a conformance walk"),
        "board" to entityValue("Sprint 24"),
        "status" to entityValue("todo"),
    )

/**
 * The envelope a submit travels in: a form identifier, the field that changed, and the values.
 *
 * `fieldId` is empty on purpose — on a submit nothing changed, which is what §16.4 says it means.
 */
private fun submission(
    formId: String,
    vararg values: Pair<String, kotlinx.serialization.json.JsonObject>,
) = kotlinx.serialization.json.buildJsonObject {
    put("formId", kotlinx.serialization.json.JsonPrimitive(formId))
    put("fieldId", kotlinx.serialization.json.JsonPrimitive(""))
    put(
        "values",
        kotlinx.serialization.json.buildJsonObject {
            values.forEach { (name, value) -> put(name, value) }
        },
    )
}

/**
 * Holds the live channel open long enough to catch what the seeded database already has waiting.
 *
 * Signing in here rather than reusing the kit's session is deliberate: the kit holds its credential
 * and decides which requests carry it, and reaching into that would be the harness lying about the
 * server again — the mistake this file already made once, when it injected a header on every request
 * and turned one check into five findings about itself.
 */
private fun recordUpdates(
    target: String,
    client: HttpClient,
): String =
    runBlocking {
        val token = signIn(target, client)
        val recorded = StringBuilder()
        withTimeoutOrNull(4_000) {
            client
                .prepareGet("$target/updates") {
                    header("Authorization", "Bearer $token")
                }.execute { response ->
                    val channel = response.bodyAsChannel()
                    while (!channel.isClosedForRead && recorded.length < 64_000) {
                        val line = channel.readUTF8Line() ?: break
                        recorded.append(line).append('\n')
                    }
                }
        }
        recorded.toString()
    }

private suspend fun signIn(
    target: String,
    client: HttpClient,
): String {
    val body =
        client
            .post("$target/submit/sign-in") {
                contentType(ContentType.Application.Json)
                setBody(
                    submission(
                        "sign-in",
                        "email" to textValue("anna@tacku.team"),
                        "password" to textValue("conformance-stand"),
                    ).toString(),
                )
            }.bodyAsText()
    return Regex("\"accessToken\"\\s*:\\s*\"([^\"]+)\"").find(body)?.groupValues?.get(1)
        ?: error("the stand could not sign in to record the update channel: $body")
}

private fun booleanValue(value: Boolean) =
    kotlinx.serialization.json.buildJsonObject {
        put("type", kotlinx.serialization.json.JsonPrimitive("boolean_value"))
        put("value", kotlinx.serialization.json.JsonPrimitive(value))
    }

// A selection sends an entity, not text. The walk sent text for a while and every select in the
// product read it as nothing chosen — which the server accepted as "no status given" and refused on
// grounds that read like the request's fault.
private fun entityValue(id: String) =
    kotlinx.serialization.json.buildJsonObject {
        put("type", kotlinx.serialization.json.JsonPrimitive("entity_value"))
        put("id", kotlinx.serialization.json.JsonPrimitive(id))
        put("title", kotlinx.serialization.json.JsonPrimitive(id))
    }

private fun textValue(text: String) =
    kotlinx.serialization.json.buildJsonObject {
        put("type", kotlinx.serialization.json.JsonPrimitive("text_value"))
        put("text", kotlinx.serialization.json.JsonPrimitive(text))
    }
