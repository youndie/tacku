package tacku.app

import io.github.youndie.kompot.kompotEngineSerializersModule
import io.github.youndie.kompot.kompotJson
import io.github.youndie.kompot.realtime.KompotRealtimeSource
import io.github.youndie.kompot.realtime.UpdateComponentMessage
import io.ktor.client.HttpClient
import io.ktor.client.request.header
import io.ktor.client.request.prepareGet
import io.ktor.client.statement.bodyAsChannel
import io.ktor.utils.io.readUTF8Line
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.serialization.json.Json
import kotlinx.serialization.modules.SerializersModule
import tacku.fields.tackuFieldsSerializersModule

/**
 * The live channel, read as what it is: a sequence of frames rather than a document.
 *
 * The topic is not passed. This client asks for its own channel and gets whatever the token it is
 * holding entitles it to — which is the whole of the server's topic rule, seen from the other end.
 * A source that named a topic would be a client asking to be told somebody else's updates, and the
 * only thing standing between that request and an answer would be the server remembering to refuse.
 *
 * Lines that are not a frame are skipped rather than repaired. A reader that glued a payload broken
 * across raw lines back together would report neither half of the damage, which is precisely the
 * mistake a first attempt at this makes — the kit's own reader says so, and this one agrees with it
 * on purpose.
 */
class Updates(
    private val baseUrl: String,
    private val token: () -> String?,
) : KompotRealtimeSource {
    private val http = HttpClient()

    // The same vocabulary the screens are decoded with, extensions included: a frame carries a
    // component, and a component this deployment added is still one of them.
    private val json: Json =
        kompotJson(
            SerializersModule {
                include(kompotEngineSerializersModule)
                include(tackuFieldsSerializersModule)
            },
        )

    override fun subscribe(topic: String): Flow<UpdateComponentMessage> =
        flow {
            val bearer = token() ?: return@flow
            http
                .prepareGet("$baseUrl/updates") {
                    header("Authorization", "Bearer $bearer")
                }.execute { response ->
                    val channel = response.bodyAsChannel()
                    while (!channel.isClosedForRead) {
                        val line = channel.readUTF8Line() ?: break
                        if (!line.startsWith(DATA)) continue
                        emit(decodeFrame(line.removePrefix(DATA)))
                    }
                }
        }

    /**
     * One frame, decoded with the vocabulary of this deployment.
     *
     * Internal rather than private so that a test can hand it a frame without opening a socket: what
     * is worth checking is the vocabulary, and a connection would only make that harder to see.
     */
    fun decodeFrame(body: String): UpdateComponentMessage = json.decodeFromString(body)

    private companion object {
        const val DATA = "data: "
    }
}
