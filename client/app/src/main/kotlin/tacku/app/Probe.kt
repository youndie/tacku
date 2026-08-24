package tacku.app

import io.github.youndie.kompot.KompotAction
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
import io.github.youndie.kompot.standard.TextComponent
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
            Navigator.resolvePrefixed(card.deeplink)
                ?: error("nothing in this client resolves \"${card.deeplink}\": opening a card does nothing")
        open(transport, opened)
        println("  opened ${card.deeplink} through ${opened.path}")

        val moved = transport.perform(card.moveUrl, card.movePayload)
        println("  moved ${card.taskId}, and the server answered ${moved::class.simpleName}")

        // And then the half a person actually looks at: the answer is a navigate, the client follows
        // it, and the board it gets back is what the screen becomes. Checked here because "the card
        // moved but the screen did not change" is a sentence about this fetch, and nothing else in
        // the client can tell the two apart.
        val reloaded = transport.screen("/screens/board")
        val column = columnOf(reloaded, card.taskId)
        println("  reloaded the board, and ${card.taskId} is now in ${column ?: "no column at all"}")

        // The read-only view over another repository's backlog, where a deployment carries one. Its
        // card is the reason this walk stopped trusting a written-down kind: the two destinations
        // that existed were forms, the third is a screen, and the client asked for the wrong shape
        // — which nothing here noticed, because nothing here opened one.
        val docs = graph.routes.firstOrNull { it.deeplink == "app://docs-board" }
        if (docs == null) {
            println("  no docs board on this deployment, so its card was not pressed")
        } else {
            val item =
                firstDeeplink(transport.screen(docs.endpoint), "app://docs-item/")
                    ?: error("the docs board carries no card, so this probe pressed nothing on it")
            val target =
                Navigator.resolvePrefixed(item)
                    ?: error("nothing in this client resolves \"$item\": opening an item does nothing")
            open(transport, target)
            println("  opened $item through ${target.path} as ${target.kind}")
        }
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

/** Fetches a destination the way the application would, which is the point: the kind is not ours. */
private suspend fun open(
    transport: Transport,
    target: Navigator.Target.Open,
) {
    when (target.kind) {
        "form" -> report(target.path, transport.form(target.path))
        else -> report(target.path, transport.screen(target.path))
    }
}

/** Whatever the node does when it is pressed, whichever component it happens to be. */
private fun actionOf(node: KompotComponent): KompotAction? =
    when (node) {
        is ColumnComponent -> node.action
        is RowComponent -> node.action
        is ButtonComponent -> node.action
        else -> null
    }

/** The first deeplink under a prefix, read out of the tree rather than written down here. */
private fun firstDeeplink(
    root: KompotComponent,
    prefix: String,
): String? {
    var found: String? = null

    fun walk(node: KompotComponent) {
        if (found != null) return
        val deeplink = (actionOf(node) as? NavigateAction)?.deeplink
        if (deeplink != null && deeplink.startsWith(prefix)) {
            found = deeplink
            return
        }
        children(node).forEach(::walk)
    }

    walk(root)
    return found
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

        // Asked of whatever carries an action rather than of a particular component. It used to
        // ask a ColumnComponent for the deeplink and a ButtonComponent for the move, and the move
        // became a row carrying an action — after which this walk found no card on a board full of
        // them, and said so to nobody, because the probe is not in the gate.
        val deeplink =
            children
                .firstNotNullOfOrNull { (actionOf(it) as? NavigateAction)?.deeplink }
                ?.takeIf { it.startsWith(Navigator.TASK_PREFIX) }
        val move = children.firstNotNullOfOrNull { actionOf(it) as? PerformAction }

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

/** Which column a task is in, read from a board the way a person reads it. */
private fun columnOf(
    root: KompotComponent,
    task: String,
): String? {
    var found: String? = null

    fun walk(
        node: KompotComponent,
        column: String?,
    ) {
        if (found != null) return
        val here =
            when (node) {
                is ColumnComponent ->
                    node.children
                        .filterIsInstance<RowComponent>()
                        .firstNotNullOfOrNull { row ->
                            row.children
                                .filterIsInstance<TextComponent>()
                                .firstOrNull()
                                ?.text
                        } ?: column
                else -> column
            }

        if (node is TextComponent && node.text.contains(task)) {
            found = here
            return
        }

        val children =
            when (node) {
                is ColumnComponent -> node.children
                is RowComponent -> node.children
                is PaginatedListComponent -> node.initialItems
                else -> emptyList()
            }
        children.forEach { walk(it, here) }
    }

    walk(root, null)
    return found
}
