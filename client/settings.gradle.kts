// The repository filter is not tidiness. An unfiltered repository that is unreachable disables
// itself for the whole resolution, and Gradle then reports "could not resolve <our artifact>"
// while the real cause is a network error against someone else's host, nested three lines down.
pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
        // Compose Multiplatform's plugin is published by JetBrains rather than to the portal.
        maven("https://maven.pkg.jetbrains.space/public/p/compose/dev")
        // viddik, the screenshot harness, is published beside kompot.
        maven("https://reposilite.kotlin.website/snapshots") {
            mavenContent { includeGroupAndSubgroups("ru.workinprogress") }
        }
    }
}

dependencyResolutionManagement {
    repositories {
        mavenCentral()
        // Compose Multiplatform pulls androidx from Google's repository, and the filter is not
        // tidiness: an unfiltered repository that becomes unreachable disables itself for the whole
        // resolution, and Gradle then blames the artefact we asked for rather than the host that
        // failed.
        google {
            mavenContent {
                includeGroupAndSubgroups("androidx")
                includeGroupAndSubgroups("com.android")
                includeGroupAndSubgroups("com.google")
            }
        }
        maven("https://maven.pkg.jetbrains.space/public/p/compose/dev")
        maven("https://reposilite.kotlin.website/snapshots") {
            mavenContent {
                includeGroup("io.github.youndie")
                includeGroupAndSubgroups("ru.workinprogress")
            }
        }
    }
}

rootProject.name = "tacku-client"

include(":fields")
include(":spec-gen")
include(":tck")
include(":shared")
include(":app")
include(":web")
