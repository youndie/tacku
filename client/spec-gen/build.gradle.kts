plugins {
    kotlin("jvm")
    kotlin("plugin.serialization")
}

val kompotVersion: String = property("kompot.version").toString()

// JVM-only and deliberately small. This module exists because the profile of a build can only be
// produced by Kotlin: KompotSpec reads the SerialDescriptors and polymorphic registrations of the
// protocol types themselves, and there is no second source of truth for them. Everything the Go
// server needs comes out of here as plain JSON.
//
// This is also where the barrier of docs/research/research-architecture.md §0 is at its narrowest,
// so the line is worth stating: reading the published Kotlin API of kompot to drive the generator
// is allowed — a foreign implementer building the same tool would read exactly this. What is not
// allowed is carrying protocol knowledge learned here into the Go handlers.
dependencies {
    implementation(project(":fields"))
    implementation("io.github.youndie:kompot-spec:$kompotVersion")
    implementation("io.github.youndie:form-standard-jvm:$kompotVersion")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")

    testImplementation(kotlin("test"))
}

kotlin {
    // 25, because that is what kompot is published for: resolution fails outright on 21 with
    // "only compatible with JVM runtime version 25 or newer". A consumer does not get to choose
    // here, and the number is worth stating rather than inheriting silently.
    jvmToolchain(25)
}

// The generated files are an input of the test just as the classes are: without this, editing or
// deleting one of them does not re-run the test, and a spec that has drifted sails through as
// UP-TO-DATE.
tasks.test {
    useJUnitPlatform()
    inputs
        .files(fileTree(rootProject.layout.projectDirectory.dir("../spec")))
        .withPropertyName("specFiles")
        .withPathSensitivity(PathSensitivity.RELATIVE)
    environment(
        "TACKU_SPEC_DIR",
        rootProject.layout.projectDirectory
            .dir("../spec")
            .asFile.absolutePath,
    )
    System.getenv("TACKU_SPEC_RECORD")?.let { environment("TACKU_SPEC_RECORD", it) }
}
