package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.form.FieldValue
import io.github.youndie.kompot.form.standard.TextValue
import io.github.youndie.kompot.forms.KompotFormResponse
import io.github.youndie.kompot.standard.PaginatedListComponent
import kotlinx.coroutines.runBlocking

/**
 * Walks a running server with the toolkit's own deserialisers and prints what came back.
 *
 * A stronger check than validating against the schema, and a different one: the schema says what is
 * allowed, this says what the code that will actually draw the screen accepts. A response can
 * satisfy the schema and still fail here — a polymorphic value in a concrete position, a
 * discriminator missing from a root — which is the class of mistake the specification warns about
 * twice.
 *
 * It is a runnable rather than a test for the reason the conformance harness is: a test that skips
 * itself when nothing is listening is a check nobody notices has stopped running.
 */
fun main() {
    val target = System.getenv("TACKU_URL") ?: "http://localhost:8477"
    val transport = Transport(target)

    runBlocking {
        println("probe: $target")

        val signIn = transport.form("/forms/sign-in")
        report("/forms/sign-in", signIn)

        val values: Map<String, FieldValue> =
            mapOf(
                "email" to TextValue("anna@tacku.team"),
                "password" to TextValue("conformance-stand"),
            )
        val session = transport.submit("/submit/sign-in", signIn.schema.formId, values)
        val access =
            (session as? io.github.youndie.kompot.auth.UpdateSessionAction)?.accessToken
                ?: error("signing in answered ${session::class.simpleName}, and a sign-in answers update_session")
        transport.useSession(access)
        println("  signed in, session replaced")

        val graph = transport.graph()
        println("graph: ${graph.routes.size} routes")

        for (route in graph.routes) {
            when (route.kind ?: "screen") {
                "form" -> report(route.endpoint, transport.form(route.endpoint))
                else -> report(route.endpoint, transport.screen(route.endpoint))
            }
        }

        println("probe: every route decoded")
    }
}

private fun report(
    path: String,
    response: KompotFormResponse,
) {
    println(
        "  $path: form ${response.schema.formId}, ${response.schema.fields.size} fields, ${count(
            response.screen,
        )} nodes",
    )
    walkPages(response.screen)
}

private fun report(
    path: String,
    component: KompotComponent,
) {
    println("  $path: screen, ${count(component)} nodes")
    walkPages(component)
}

private fun count(component: KompotComponent): Int = 1 + children(component).sumOf { count(it) }

/**
 * A list's first page arrives inside the tree and the rest behind an address, so the walk has to
 * follow it: a page that only ever renders when somebody scrolls is a page nobody has decoded.
 */
private fun walkPages(component: KompotComponent) {
    if (component is PaginatedListComponent) {
        val next = component.loadMoreAction?.url
        if (next != null) println("    list ${component.id}: ${component.initialItems.size} items, more at $next")
    }
    children(component).forEach { walkPages(it) }
}

private fun children(component: KompotComponent): List<KompotComponent> =
    when (component) {
        is io.github.youndie.kompot.standard.ColumnComponent -> component.children
        is io.github.youndie.kompot.standard.RowComponent -> component.children
        is PaginatedListComponent -> component.initialItems
        else -> emptyList()
    }
