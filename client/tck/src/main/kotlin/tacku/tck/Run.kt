package tacku.tck

import io.github.youndie.kompot.tck.RemoteTckTransport
import io.github.youndie.kompot.tck.TckConfig
import io.github.youndie.kompot.tck.TckRunner
import io.ktor.client.HttpClient
import io.ktor.client.engine.cio.CIO
import io.ktor.client.plugins.defaultRequest
import io.ktor.client.request.header
import kotlinx.coroutines.runBlocking
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

    // The token is held by the harness and sent on EVERY request, and that is a known lie to the
    // kit rather than a feature.
    //
    // The kit obtains credentials for itself through `loginPath`, and this build has no form to log
    // in with, so there is no supported way to hand it one. Injecting the header covers the gap for
    // seven checks and breaks the eighth: `auth-required` probes an endpoint deliberately without a
    // token, the header goes on anyway, and the 200 it then sees is real. Those findings are the
    // harness lying, not the server misbehaving — verified by walking anonymously, where the same
    // three endpoints answer 401 correctly.
    //
    // The honest fix is a login endpoint, which is B-09. Until then the run prints what it is doing
    // so nobody reads the three findings as a defect.
    val token = System.getenv("TACKU_TCK_TOKEN").orEmpty()
    val client =
        HttpClient(CIO) {
            if (token.isNotEmpty()) {
                defaultRequest { header("Authorization", "Bearer $token") }
            }
        }
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
                    loginPath = null,
                    // The idempotency check performs a real operation — there is no way to reach
                    // 400 and 409 otherwise, the handler refusing on the merits long before it
                    // looks at the header — so it only runs when a payload is given. This walk is
                    // against a throwaway database, which is where the kit's own README says to
                    // run it.
                    submitPayloads = mapOf("/submit/new-task" to newTaskPayload()),
                ),
            ).run()
        }

    if (token.isNotEmpty()) {
        println(
            "note: this walk sends a token on every request, including the probes auth-required " +
                "means to be anonymous. Its findings are an artefact of that and not a defect; " +
                "see B-14.",
        )
        println()
    }
    println(TckGate.describe(report))

    val verdict = TckGate.judge(report)
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
    kotlinx.serialization.json.buildJsonObject {
        put("formId", kotlinx.serialization.json.JsonPrimitive("task_create"))
        put("fieldId", kotlinx.serialization.json.JsonPrimitive(""))
        put(
            "values",
            kotlinx.serialization.json.buildJsonObject {
                put("title", textValue("Filed by a conformance walk"))
                put("board", textValue("Sprint 24"))
                put("status", textValue("todo"))
            },
        )
    }

private fun textValue(text: String) =
    kotlinx.serialization.json.buildJsonObject {
        put("type", kotlinx.serialization.json.JsonPrimitive("text_value"))
        put("text", kotlinx.serialization.json.JsonPrimitive(text))
    }
