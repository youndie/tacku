plugins {
    kotlin("multiplatform")
    id("org.jetbrains.compose")
    id("org.jetbrains.kotlin.plugin.compose")
}

// The product's human surface, and the only one that ships.
//
// A page rather than a window: the desktop client is an instrument for finding out whether the
// server is right, and a person who has installed nothing has never been able to use this product at
// all. It could not exist until the toolkit published for `wasmJs` (kompot#42) — which is why this
// module is thin: everything it draws lives in `:shared`, and what is here is a viewport and an
// address.
kotlin {
    @OptIn(org.jetbrains.kotlin.gradle.ExperimentalWasmDsl::class)
    wasmJs {
        browser {
            commonWebpackConfig { outputFileName = "tacku.js" }
        }
        binaries.executable()
    }

    sourceSets {
        val wasmJsMain by getting {
            // The same directory the desktop reads out of its jar. Declared as a resource root
            // rather than copied: a copy task writing into another task's output is a build that
            // works until Gradle runs them in the other order, and a second copy of a binary is a
            // second thing to keep in step.
            resources.srcDir(rootProject.file("resources"))

            dependencies {
                implementation(project(":shared"))
                implementation(compose.runtime)
                implementation(compose.ui)
            }
        }
    }
}
