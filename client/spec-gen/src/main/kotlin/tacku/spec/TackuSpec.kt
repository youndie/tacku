package tacku.spec

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
    // The toolkit's own list, and nothing added.
    //
    // It used to be `KompotToolkitSpec.modules + formStandard()`, because kompot published
    // form-standard as a library and described it nowhere, so every consumer redeclared it — the
    // finding this project reported as kompot#2. The toolkit now describes it, and the declaration
    // that used to live here is gone rather than kept as a harmless duplicate: the generator
    // refuses a wire type declared twice, which is how the duplication announced itself the moment
    // the version moved.
    val modules: List<KompotSpecModule> = KompotToolkitSpec.modules

    fun generate(): List<GeneratedSchema> = KompotSpec.generateAll(modules)

    fun profile(schemas: List<GeneratedSchema>): JsonObject = KompotSpec.profile(schemas)
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
    private val json =
        Json {
            prettyPrint = true
            prettyPrintIndent = "  "
        }

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
