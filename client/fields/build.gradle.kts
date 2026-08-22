plugins {
    kotlin("jvm")
    kotlin("plugin.serialization")
}

val kompotVersion: String = property("kompot.version").toString()

// The wire types this deployment adds to the vocabulary, and the only module that has any.
//
// It exists as a module of its own because two halves need it and neither is the other's: the spec
// generator reads its SerialDescriptors to put the names in the profile, and the client needs the
// classes to draw them. Declaring them inside either one would make the other depend on a thing it
// has no business knowing.
dependencies {
    api("io.github.youndie:kompot-core:$kompotVersion")
    api("io.github.youndie:form-core-jvm:$kompotVersion")
    implementation("io.github.youndie:kompot-spec:$kompotVersion")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")
}

kotlin {
    jvmToolchain(25)
}
