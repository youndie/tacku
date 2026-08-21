plugins {
    kotlin("jvm")
}

val kompotVersion: String = property("kompot.version").toString()

// The gate takes a report, not a server, so this module has no transport dependency and no
// coroutines: it can be built and tested long before there is anything to run the kit against.
dependencies {
    api("io.github.youndie:kompot-tck:$kompotVersion")

    testImplementation(kotlin("test"))
}

kotlin {
    jvmToolchain(25)
}

tasks.test {
    useJUnitPlatform()
}
