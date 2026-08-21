package tacku.spec

import io.github.youndie.kompot.spec.KompotProtocol
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The committed spec against what the generator prints now.
 *
 * This is the only thing standing between a kompot upgrade and a Go server validating responses
 * against a contract that no longer exists. Regenerate with:
 *
 *     TACKU_SPEC_RECORD=true ./gradlew :spec-gen:test
 */
class SpecGoldenTest {

    @Test
    fun `committed spec matches the generator`() {
        val expected = SpecOutput.files()

        if (SpecOutput.recordMode) {
            SpecOutput.write()
        }

        val directory = SpecOutput.directory
        assertTrue(directory.isDirectory, "no spec directory at ${directory.absolutePath}")

        val onDisk = directory.listFiles { f -> f.name.endsWith(".json") }.orEmpty().map { it.name }.toSet()
        assertEquals(expected.keys, onDisk, "the set of spec files on disk differs from the generated set")

        expected.forEach { (name, document) ->
            assertEquals(
                SpecOutput.render(document),
                File(directory, name).readText(),
                "$name has drifted from the generator; regenerate with ${SpecOutput.RECORD_ENV}=true",
            )
        }
    }

    @Test
    fun `the profile lists every module of this build`() {
        val profile = SpecOutput.files().getValue(KompotProtocol.PROFILE_FILE_NAME)
        val declared = TackuSpec.modules.map { it.name }
        val listed = profile["x-kompot-modules"]!!.let { element ->
            (element as kotlinx.serialization.json.JsonArray).map { (it as kotlinx.serialization.json.JsonPrimitive).content }
        }

        assertEquals(declared, listed)
        assertTrue("form-standard" in listed, "form-standard is what the toolkit's own spec set leaves out; it must be here")
    }

    /**
     * Without this the check above would be vacuous in the direction that matters: a profile is a
     * closed list, and a closed list is only worth having if something falls outside it.
     */
    @Test
    fun `the profile is closed`() {
        val profile = SpecOutput.files().getValue(KompotProtocol.PROFILE_FILE_NAME)
        val text = profile.toString()

        assertTrue("\"text_field\"" in text, "a type this build does declare is missing from the profile")
        assertTrue("\"tacku_nonexistent_field\"" !in text, "the profile accepted a type nobody declared")
    }
}
