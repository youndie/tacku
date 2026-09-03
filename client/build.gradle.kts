// Repositories are declared once, in settings.gradle.kts. A `subprojects { repositories { … } }`
// block here would silently win over that declaration and drop the filtered snapshot repository —
// the first version of this file did exactly that, and the failure read "Could not find
// io.github.youndie:kompot-spec", which points at the artefact rather than at the cause.
plugins {
    kotlin("jvm") version "2.4.10" apply false
    kotlin("plugin.serialization") version "2.4.10" apply false
    id("org.jlleitschuh.gradle.ktlint") version "14.2.0" apply false
    id("org.jetbrains.compose") version "1.12.0" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.10" apply false
    id("com.google.devtools.ksp") version "2.3.11" apply false
    id("ru.workinprogress.viddik") version "0.1.2.13" apply false
}

// The formatter is a gate, not a convenience: the Go half has gofmt in `make check`, and a rule
// enforced on one half of a two-language repository is a rule that gets argued about on the other.
subprojects {
    apply(plugin = "org.jlleitschuh.gradle.ktlint")
}
