// The repository filter is not tidiness. An unfiltered repository that is unreachable disables
// itself for the whole resolution, and Gradle then reports "could not resolve <our artifact>"
// while the real cause is a network error against someone else's host, nested three lines down.
dependencyResolutionManagement {
    repositories {
        mavenCentral()
        maven("https://reposilite.kotlin.website/snapshots") {
            mavenContent { includeGroup("io.github.youndie") }
        }
    }
}

rootProject.name = "tacku-client"

include(":spec-gen")
include(":tck")
