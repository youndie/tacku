plugins {
    kotlin("multiplatform")
    kotlin("plugin.serialization")
}

val kompotVersion: String = property("kompot.version").toString()

// The wire types this deployment adds to the vocabulary, and the only module that has any.
//
// It exists as a module of its own because two halves need it and neither is the other's: the spec
// generator reads its SerialDescriptors to put the names in the profile, and the client needs the
// classes to draw them. Declaring them inside either one would make the other depend on a thing it
// has no business knowing.
//
// Multiplatform because a wire type is not a platform's business at all. It was `jvm()` alone for as
// long as the only client was a desktop one — and that is exactly the shape of omission the toolkit
// had five modules of: a target set that was never decided, only inherited from whoever asked first.
kotlin {
    jvmToolchain(25)

    jvm()

    @OptIn(org.jetbrains.kotlin.gradle.ExperimentalWasmDsl::class)
    wasmJs { browser() }

    sourceSets {
        commonMain.dependencies {
            api("io.github.youndie:kompot-core:$kompotVersion")
            api("io.github.youndie:form-core:$kompotVersion")
            implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")
        }
        jvmMain.dependencies {
            implementation("io.github.youndie:kompot-spec:$kompotVersion")
        }
    }
}
