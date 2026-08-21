import org.jetbrains.compose.desktop.application.dsl.TargetFormat

plugins {
    kotlin("jvm")
    kotlin("plugin.serialization")
    id("org.jetbrains.compose")
    id("org.jetbrains.kotlin.plugin.compose")
}

val kompotVersion: String = property("kompot.version").toString()

// The client is a measuring instrument before it is a product.
//
// It is the most exercised part of the system — somebody else wrote and tested it — so a
// disagreement it shows is almost certainly on the server's side. That is the discipline: when
// something does not render, find out which of the two is wrong by the specification before
// changing either.
//
// It is also the only thing that can settle several assumptions the server made alone: under what
// names a reload carries field values, and what a client actually does with a deeplink it does not
// know.
dependencies {
    implementation(compose.desktop.currentOs)
    implementation(compose.material3)

    // The wire types are declared even though the client modules depend on them: they arrive as
    // `implementation` there, so a consumer that needs to name a KompotFormResponse cannot see one.
    implementation("io.github.youndie:kompot-core:$kompotVersion")
    implementation("io.github.youndie:kompot-standard:$kompotVersion")
    implementation("io.github.youndie:kompot-forms:$kompotVersion")
    implementation("io.github.youndie:kompot-navigation:$kompotVersion")
    implementation("io.github.youndie:form-core:$kompotVersion")
    implementation("io.github.youndie:kompot-auth:$kompotVersion")
    implementation("io.github.youndie:kompot-commands:$kompotVersion")

    implementation("io.github.youndie:kompot-client:$kompotVersion")
    implementation("io.github.youndie:kompot-forms-client:$kompotVersion")
    implementation("io.github.youndie:kompot-ds-material-compose:$kompotVersion")
    implementation("io.github.youndie:form-standard:$kompotVersion")

    implementation("io.ktor:ktor-client-cio:3.5.2")
    implementation("io.ktor:ktor-client-content-negotiation:3.5.2")
    implementation("io.ktor:ktor-serialization-kotlinx-json:3.5.2")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")

    testImplementation(kotlin("test"))
}

kotlin {
    jvmToolchain(25)
}

compose.desktop {
    application {
        mainClass = "tacku.app.MainKt"
        nativeDistributions {
            targetFormats(TargetFormat.Dmg)
            packageName = "tacku"
            packageVersion = "1.0.0"
        }
    }
}

tasks.test {
    useJUnitPlatform()
}

// Walks a running server with the toolkit's deserialisers. `make probe` starts one and calls this.
tasks.register<JavaExec>("probe") {
    group = "verification"
    description = "Decode every screen of a running tacku with the toolkit's own parsers"
    mainClass.set("tacku.app.ProbeKt")
    classpath = sourceSets["main"].runtimeClasspath
}
