plugins {
    kotlin("jvm")
}

val kompotVersion: String = property("kompot.version").toString()

// The gate takes a report, not a server, so this module has no transport dependency and no
// coroutines: it can be built and tested long before there is anything to run the kit against.
dependencies {
    api("io.github.youndie:kompot-tck:$kompotVersion")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.11.0")
    implementation("io.ktor:ktor-client-cio:3.5.2")
    // Ktor logs through SLF4J; without a binding every run opens with three lines of warning, and a
    // report people are meant to read should not start with noise about logging.
    runtimeOnly("org.slf4j:slf4j-nop:2.0.18")

    testImplementation(kotlin("test"))
}

kotlin {
    jvmToolchain(25)
}

tasks.test {
    useJUnitPlatform()
}

// A runnable rather than a test. A test that skips itself when no server is listening is the
// shape of a check nobody notices has stopped running; this one either walks a server or does not
// exist. `make tck` starts the server and calls it.
tasks.register<JavaExec>("tck") {
    group = "verification"
    description = "Walk a running tacku over HTTP with the conformance kit"
    mainClass.set("tacku.tck.RunKt")
    classpath = sourceSets["main"].runtimeClasspath
    systemProperty("tacku.tck.target", providers.gradleProperty("target").getOrElse("http://localhost:8080"))
    systemProperty(
        "tacku.tck.spec",
        rootProject.layout.projectDirectory
            .dir("../spec")
            .asFile.absolutePath,
    )
}
