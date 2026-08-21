package tacku.spec

import io.github.youndie.kompot.form.standard.formStandardSerializersModule
import io.github.youndie.kompot.spec.GeneratedSchema
import io.github.youndie.kompot.spec.KompotProtocol
import io.github.youndie.kompot.spec.KompotSpec
import io.github.youndie.kompot.spec.KompotSpecModule
import io.github.youndie.kompot.spec.KompotToolkitSpec
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject

/**
 * The spec of this build: the ten protocol modules of the toolkit plus form-standard.
 *
 * The profile is what makes conformance checkable at all — TckConfig assembles its schema map from
 * the module list inside the profile rather than from a directory listing — and only Kotlin can
 * produce it, because the generator reads the SerialDescriptors of the wire types themselves.
 */
object TackuSpec {

    val modules: List<KompotSpecModule> = KompotToolkitSpec.modules + formStandard()

    fun generate(): List<GeneratedSchema> = KompotSpec.generateAll(modules)

    fun profile(schemas: List<GeneratedSchema>): JsonObject = KompotSpec.profile(schemas)

    /**
     * form-standard is a module of the toolkit, but the toolkit's spec set does not describe it:
     * kompot-spec depends on ten protocol modules and this is not one of them. So every application
     * that uses standard form fields has to declare this module itself — including the annotations
     * for the metadata keys the protocol reserves in SPEC.md §9.7.
     *
     * That duplication is the finding of docs/backlog/B-03: protocol knowledge repeated verbatim by
     * every consumer lives in the wrong place. Kept visible here rather than tidied away, so that
     * the day it moves into kompot this function disappears instead of quietly diverging.
     */
    private fun formStandard() =
        KompotSpecModule(
            name = "form-standard",
            description = "The standard form fields, rules, values and conditions",
            serializersModule = formStandardSerializersModule,
            annotations =
                mapOf(
                    "FieldValueEntityValue" to mapOf("rawMetadata" to KompotSpec.reservedMetadata()),
                    "ValidationRuleMaxAmountFromField" to
                        mapOf(
                            "balanceMetadataKey" to
                                KompotSpec.constrained(
                                    null,
                                    "The key in the chosen entity_value's rawMetadata the remaining amount is read " +
                                        "from. Defaults to \"${KompotProtocol.METADATA_KEY_BALANCE}\"",
                                ),
                        ),
                ),
        )
}

/**
 * Where the generated files live and how they are compared.
 *
 * They are committed rather than produced into a build directory, because the Go half of this
 * repository cannot run Kotlin: it reads finished JSON. A committed artefact can drift from its
 * generator, so the drift is what the test watches.
 */
object SpecOutput {

    const val RECORD_ENV: String = "TACKU_SPEC_RECORD"
    private const val DIR_ENV = "TACKU_SPEC_DIR"

    // Two spaces and a trailing newline: the same shape a schema file has in kompot, so a diff
    // between the two is about content rather than formatting.
    private val json = Json { prettyPrint = true; prettyPrintIndent = "  " }

    val directory: java.io.File
        get() = java.io.File(System.getenv(DIR_ENV) ?: "../../spec")

    val recordMode: Boolean
        get() = System.getenv(RECORD_ENV)?.toBoolean() == true

    fun render(document: JsonObject): String = json.encodeToString(JsonObject.serializer(), document) + "\n"

    fun files(): Map<String, JsonObject> {
        val schemas = TackuSpec.generate()
        return schemas.associate { it.fileName to it.document } +
            (KompotProtocol.PROFILE_FILE_NAME to TackuSpec.profile(schemas))
    }

    fun write() {
        directory.mkdirs()
        files().forEach { (name, document) ->
            java.io.File(directory, name).writeText(render(document))
        }
    }
}
