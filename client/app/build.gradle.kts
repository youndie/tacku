import org.jetbrains.compose.desktop.application.dsl.TargetFormat

plugins {
    kotlin("jvm")
    kotlin("plugin.serialization")
    id("org.jetbrains.compose")
    id("org.jetbrains.kotlin.plugin.compose")
    id("com.google.devtools.ksp")
    id("ru.workinprogress.viddik")
}

// The screenshots are the only tests here that look at pixels, and what they watch is the states
// nobody exercises by hand: an unknown component, an empty column, a field in error. Those are what
// break unnoticed, because nothing about a screen that renders says which branch it took.
viddik {
    jvmTarget.set("25")
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
    implementation(project(":shared"))
    implementation(project(":fields"))
    implementation(compose.desktop.currentOs)

    // What provides `Dispatchers.Main` on the desktop, and its absence is why the application had
    // never actually run.
    //
    // Nothing here needed it until the window opened: the screenshot tests render composables in a
    // harness of their own and the probe only decodes bodies, so `main()` had been called by
    // nobody. The first launch put a dialog on top of a correctly rendered sign-in screen — the
    // screen was right and the thing drawing it could not switch to the main thread.
    //
    // A test had already met this and it was read as a quirk of testing: FilterWireTest sets a main
    // dispatcher because "a plain JVM test has none". That was the application saying so a week
    // early.
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-swing:1.11.0")
    implementation(compose.material3)

    // The wire types are declared even though the client modules depend on them: they arrive as
    // `implementation` there, so a consumer that needs to name a KompotFormResponse cannot see one.
    implementation("io.github.youndie:kompot-core:$kompotVersion")
    implementation("io.github.youndie:kompot-standard:$kompotVersion")
    implementation("io.github.youndie:kompot-forms:$kompotVersion")
    implementation("io.github.youndie:kompot-navigation:$kompotVersion")
    // The live channel: the wire type of a frame, and the client-side provider that applies one to
    // the tree. Both live here rather than in the toolkit's core because a build that does not open
    // a channel should not carry either.
    implementation("io.github.youndie:kompot-realtime:$kompotVersion")
    implementation("io.github.youndie:form-core:$kompotVersion")
    implementation("io.github.youndie:kompot-auth:$kompotVersion")
    implementation("io.github.youndie:kompot-commands:$kompotVersion")

    implementation("io.github.youndie:kompot-client:$kompotVersion")
    implementation("io.github.youndie:kompot-forms-client:$kompotVersion")
    implementation("io.github.youndie:kompot-ds-material-compose:$kompotVersion")
    implementation("io.github.youndie:form-standard:$kompotVersion")

    implementation("io.ktor:ktor-client-cio:3.5.2")
    implementation("org.jetbrains.kotlinx:kotlinx-datetime:0.7.1")
    implementation("io.ktor:ktor-client-content-negotiation:3.5.2")
    implementation("io.ktor:ktor-serialization-kotlinx-json:3.5.2")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")

    testImplementation(kotlin("test"))

    // An engine that answers for the server, so a refusal can be made to happen on purpose. The
    // session this product hands out lives five minutes; what happens at minute six is not
    // observable any other way without waiting five.
    testImplementation("io.ktor:ktor-client-mock:3.5.2")

    // The wizard's wire types, for tests only: this deployment has no scenario endpoints yet
    // (B-39), so the client draws no wizard and the product does not need them. What the tests
    // need them for is the published shape of `wizard_screen` itself — the subject of B-31 — and
    // reading it from the artefact rather than from the schema file is the second half of the same
    // question: a field that is absent from both is absent from the contract.
    testImplementation("io.github.youndie:kompot-wizard:$kompotVersion")

    // Compose's own test harness, because one thing here cannot be checked from a JSON body: a
    // filter reaches the server only if the toolkit re-requests when a value changes, and that
    // behaviour lives in a composition. A body can name the right address and the value still
    // never travel.
    @OptIn(org.jetbrains.compose.ExperimentalComposeLibrary::class)
    testImplementation(compose.uiTest)
    testImplementation(compose.desktop.currentOs)
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
