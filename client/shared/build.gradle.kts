plugins {
    kotlin("multiplatform")
    kotlin("plugin.serialization")
    id("org.jetbrains.compose")
    id("org.jetbrains.kotlin.plugin.compose")
    id("com.google.devtools.ksp")
}

val kompotVersion: String = property("kompot.version").toString()

// Everything the product draws, on whichever platform draws it.
//
// It was one JVM module until the browser became possible, and the split is along the only seam that
// exists: a window and a page are different, and nothing else here is. What stayed behind in `app`
// is the desktop window and the probe; what came here is the screens, the design system, the
// transport and the navigator — the parts that have no opinion about where they run.
//
// The screenshots deliberately did not move. They live beside the JVM entry point because that is
// where the harness runs, and a golden is a picture of this code drawn by that harness; moving them
// into a multiplatform module would have meant proving the harness works there before proving
// anything else.
kotlin {
    jvmToolchain(25)

    jvm()

    @OptIn(org.jetbrains.kotlin.gradle.ExperimentalWasmDsl::class)
    wasmJs { browser() }

    sourceSets {
        commonMain.dependencies {
            implementation(project(":fields"))
            implementation(compose.runtime)
            implementation(compose.foundation)
            implementation(compose.material3)
            implementation(compose.ui)

            implementation("io.github.youndie:kompot-core:$kompotVersion")
            implementation("io.github.youndie:kompot-standard:$kompotVersion")
            implementation("io.github.youndie:kompot-forms:$kompotVersion")
            implementation("io.github.youndie:kompot-navigation:$kompotVersion")
            implementation("io.github.youndie:kompot-realtime:$kompotVersion")
            implementation("io.github.youndie:form-core:$kompotVersion")
            implementation("io.github.youndie:kompot-auth:$kompotVersion")
            implementation("io.github.youndie:kompot-commands:$kompotVersion")
            implementation("io.github.youndie:kompot-client:$kompotVersion")
            implementation("io.github.youndie:kompot-forms-client:$kompotVersion")
            implementation("io.github.youndie:kompot-ds-material-compose:$kompotVersion")
            implementation("io.github.youndie:form-standard:$kompotVersion")

            implementation("io.ktor:ktor-client-core:3.5.2")
            implementation("io.ktor:ktor-client-content-negotiation:3.5.2")
            implementation("io.ktor:ktor-serialization-kotlinx-json:3.5.2")
            implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")
            implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.11.0")

            // Dates, because the date field offers "next Friday" and a browser has no java.time.
            // The extension was written against the JVM's calendar for as long as the JVM was the
            // only place it ran — the same shape of omission as a module that declared one target.
            implementation("org.jetbrains.kotlinx:kotlinx-datetime:0.7.1")
        }

        jvmMain {
            // One directory of font files, read from two places: a jar on the desktop and the
            // page's own distribution in a browser. Kept at the client's root rather than inside
            // either, because a file copied into two source trees is a file that will differ.
            resources.srcDir(rootProject.file("resources"))

            dependencies {
                implementation("io.ktor:ktor-client-cio:3.5.2")
            }
        }

        // The browser's own engine. Ktor picks an engine off the classpath when none is named, which
        // is why the transport does not name one: the same code takes CIO on the desktop and the
        // browser's fetch here.
        val wasmJsMain by getting {
            dependencies {
                implementation("io.ktor:ktor-client-js:3.5.2")
            }
        }
    }
}
