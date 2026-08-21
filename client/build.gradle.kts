// Repositories are declared once, in settings.gradle.kts. A `subprojects { repositories { … } }`
// block here would silently win over that declaration and drop the filtered snapshot repository —
// the first version of this file did exactly that, and the failure read "Could not find
// io.github.youndie:kompot-spec", which points at the artefact rather than at the cause.
plugins {
    kotlin("jvm") version "2.4.10" apply false
    kotlin("plugin.serialization") version "2.4.10" apply false
}
