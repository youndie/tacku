package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.form.FormFieldDefinition
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.descriptors.StructureKind
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import tacku.fields.tackuFieldsSerializersModule
import java.io.File
import kotlin.test.Test
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * A name this build declares in its profile is a name this client can decode, and — for a component
 * — a name this client can draw.
 *
 * Each wire type of a deployment lives in three places, and each is a different half of the same
 * statement: the profile declares it to anything that validates, the serialisers module decodes it,
 * the renderer registry draws it. Missing from the second, a declared type fails the parse or
 * degrades; missing from the third, it decodes and then draws as a placeholder — our own client
 * producing exactly the picture a client that never heard of the type would produce, which is the
 * failure easiest to mistake for the protocol working as intended.
 *
 * The check runs from the profile outwards, because that is the direction with an artefact at the
 * start of it. The other direction — registered here and declared nowhere — is held on the Go side,
 * where `render/vocabulary_test.go` compares every discriminator the server builds against the same
 * file; a type this client could decode but the server may not send is not a wire type at all.
 */
class ExtensionRegistryTest {
    private val transport = Transport("http://localhost:0")

    @Test
    fun `every extension the profile declares can be decoded, and every component drawn`() {
        val components = extensionsOf("KompotComponent")
        val fields = extensionsOf("FormFieldDefinition")

        // Targets counted rather than assumed: a walk that finds nothing passes in silence, and a
        // profile read from the wrong place, or read wrongly, would find nothing.
        assertTrue(components.size >= 2, "the profile declares ${components.size} component extensions")
        assertTrue(fields.isNotEmpty(), "the profile declares no field extensions")

        for (name in fields) {
            assertNotNull(
                tackuFieldsSerializersModule.getPolymorphic(FormFieldDefinition::class, name),
                "$name is declared by this build and nothing here decodes it: the whole form would be lost",
            )
        }

        val renderers = tackuRenderers().keys
        for (name in components) {
            val deserializer =
                assertNotNull(
                    tackuFieldsSerializersModule.getPolymorphic(KompotComponent::class, name),
                    "$name is declared by this build and nothing here decodes it",
                )
            // The body is built from the descriptor rather than written out, so that a type added
            // later is covered by this test on the day it is added rather than on the day somebody
            // remembers to write a fixture for it.
            val decoded = transport.decodeScreen(sampleOf(name, deserializer.descriptor))
            assertTrue(
                decoded::class in renderers,
                "$name decodes as ${decoded::class.simpleName} and no renderer claims it: it would draw" +
                    " as the placeholder a client without the extension sees",
            )
        }
    }

    /** The names one hierarchy of the profile adds on top of its modules. */
    private fun extensionsOf(hierarchy: String): List<String> {
        val file = File(System.getenv("TACKU_SPEC_DIR") ?: "../../spec", "kompot.profile.schema.json")
        assertTrue(file.isFile, "no profile at ${file.absolutePath}")

        val profile = Json.parseToJsonElement(file.readText()) as JsonObject
        val defs = profile["\$defs"] as JsonObject
        val declared = (defs[hierarchy] as JsonObject)["x-kompot-extensions"] as? JsonArray
        return declared.orEmpty().map { (it as JsonPrimitive).content }
    }

    /**
     * The smallest body of a type: its discriminator plus every element the class has no default
     * for.
     */
    private fun sampleOf(
        wireType: String,
        descriptor: SerialDescriptor,
    ): String {
        val fields = mutableListOf("\"type\":\"$wireType\"")
        for (index in 0 until descriptor.elementsCount) {
            if (descriptor.isElementOptional(index)) continue
            val element = descriptor.getElementDescriptor(index)
            val value =
                when (element.kind) {
                    PrimitiveKind.STRING -> "\"x\""
                    PrimitiveKind.INT -> "1"
                    PrimitiveKind.BOOLEAN -> "false"
                    StructureKind.LIST -> "[]"
                    else -> "null"
                }
            fields += "\"${descriptor.getElementName(index)}\":$value"
        }
        return fields.joinToString(",", prefix = "{", postfix = "}")
    }
}
