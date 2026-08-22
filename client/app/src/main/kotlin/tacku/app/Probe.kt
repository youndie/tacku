package tacku.app

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.commands.PerformAction
import io.github.youndie.kompot.form.FieldValue
import io.github.youndie.kompot.form.standard.TextValue
import io.github.youndie.kompot.forms.KompotFormResponse
import io.github.youndie.kompot.standard.ButtonComponent
import io.github.youndie.kompot.standard.ColumnComponent
import io.github.youndie.kompot.standard.NavigateAction
import io.github.youndie.kompot.standard.PaginatedListComponent
import io.github.youndie.kompot.standard.RowComponent
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

        // Decoding is not doing. Every check before this one asked whether a body parses and whether
        // a picture matches; none pressed anything, and two of the product's three verbs — open a
        // card, move a card — turned out to be dead in the running application while every check was
        // green. So the probe now walks the two intents a board is for, through the same client code
        // the application runs.
        val board = transport.screen("/screens/board")
        val card = firstCard(board) ?: error("the board carries no card, so this probe pressed nothing")

        val opened =
            Navigator.resolveTaskPath(card.deeplink)
                ?: error("nothing in this client resolves \"${card.deeplink}\": opening a card does nothing")
        report(opened, transport.form(opened))
        println("  opened ${card.deeplink} through $opened")

        val moved = transport.perform(card.moveUrl, card.movePayload)
        println("  moved ${card.taskId}, and the server answered ${moved::class.simpleName}")
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

/** What a card offers: where it opens, and where its move posts. */
private data class Card(
    val taskId: String,
    val deeplink: String,
    val moveUrl: String,
    val movePayload: Map<String, FieldValue>,
)

/**
 * The first card on the board that offers both, found by walking the tree the server sent.
 *
 * Read from the response rather than written down here on purpose: a probe that knows the addresses
 * in advance keeps passing after the server stops sending them.
 */
private fun firstCard(root: KompotComponent): Card? {
    var found: Card? = null

    fun walk(node: KompotComponent) {
        if (found != null) return
        val children =
            when (node) {
                is RowComponent -> node.children
                is ColumnComponent -> node.children
                is PaginatedListComponent -> node.initialItems
                else -> emptyList()
            }

        val deeplink =
            children
                .filterIsInstance<ColumnComponent>()
                .firstNotNullOfOrNull { (it.action as? NavigateAction)?.deeplink }
                ?.takeIf { it.startsWith(Navigator.TASK_PREFIX) }
        val move = children.filterIsInstance<ButtonComponent>().firstNotNullOfOrNull { it.action as? PerformAction }

        if (deeplink != null && move != null) {
            found =
                Card(
                    taskId = deeplink.removePrefix(Navigator.TASK_PREFIX),
                    deeplink = deeplink,
                    moveUrl = move.url,
                    movePayload = move.payload,
                )
            return
        }

        children.forEach(::walk)
    }

    walk(root)
    return found
}
