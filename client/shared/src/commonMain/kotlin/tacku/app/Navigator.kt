package tacku.app

import io.github.youndie.kompot.KompotAction
import io.github.youndie.kompot.KompotActionHandler
import io.github.youndie.kompot.auth.UpdateSessionAction
import io.github.youndie.kompot.commands.PerformAction
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.forms.SubmitFormAction
import io.github.youndie.kompot.navigation.ScreenRoute
import io.github.youndie.kompot.standard.CloseAction
import io.github.youndie.kompot.standard.CopyTextAction
import io.github.youndie.kompot.standard.LoadPageAction
import io.github.youndie.kompot.standard.NavigateAction
import kotlinx.coroutines.CoroutineScope

/**
 * Where the client's own decisions live: what to do with an intent, and where a deeplink leads.
 *
 * Every answer from the server travels through here, including the answers to submits — §16.4 says a
 * submit answers an action and the client runs it through the same chain as any other, which is how
 * signing in ends with a session and creating a task ends on a board.
 */
class Navigator(
    private val transport: Transport,
    private val scope: CoroutineScope,
    // Kept so that signing out starts again the same way signing in did.
    private val door: Door? = null,
    private val show: (Screen) -> Unit,
) {
    private var routes: List<ScreenRoute> = emptyList()

    /** The sign-in screen, which is the one place a client has to know without a graph. */
    fun start(door: Door?) {
        scope.launchCatching(onFailure = { show(Screen.Failed(it)) }) {
            // The platform's own door first. In a page that is a redirect to the identity provider,
            // which is the product's way in; on the desktop there is none, and the instrument falls
            // back to the form its stand serves. A shipped server serves no such form, so the
            // fallback there ends in a refusal rather than a screen — which is the honest outcome:
            // the instrument is not a way into production.
            if (door != null) {
                val token = door.resume()
                if (token == null) {
                    door.open()
                    return@launchCatching
                }
                transport.useSession(token)
                loadGraph()
                follow(DEFAULT_SCREEN)
                return@launchCatching
            }

            open(SIGN_IN_PATH, kind = "form")
        }
    }

    fun handler(
        controller: FormController,
        formId: String?,
    ): KompotActionHandler =
        KompotActionHandler { action ->
            trace("intent ${action::class.simpleName}")
            dispatch(action, controller, formId)
        }

    /**
     * What the client did, in order, when `TACKU_TRACE` is set.
     *
     * Off by default and one line per step. It exists because a chain whose every link was proved
     * to work separately still did not work together, and at that point the only measurement left is
     * the running application saying what it actually did.
     */
    private fun trace(what: String) {
        if (traceEnabled) {
            println("tacku: $what")
        }
    }

    private fun dispatch(
        action: KompotAction,
        controller: FormController,
        formId: String?,
    ) {
        scope.launchCatching(onFailure = { show(Screen.Failed(it)) }) {
            when (action) {
                is NavigateAction -> follow(action.deeplink)

                is SubmitFormAction -> {
                    // Only the fields a person can currently see. A field whose condition is not
                    // met is not rendered, not validated and must not travel in the payload (§9.4)
                    // — even if a value was typed into it before something else hid it.
                    // A field with no value yet is left out rather than sent as null. §1.4: a field
                    // whose schema does not allow null must not arrive as one — it is omitted.
                    val values =
                        controller.fieldsState.value
                            .filterKeys { controller.isFieldVisible(it) }
                            .mapNotNull { (id, state) -> state.value?.let { id to it } }
                            .toMap()
                    val answer = transport.submit(submitPathFor(action.formId), action.formId, values)
                    dispatchNow(answer, controller, formId)
                }

                is PerformAction -> {
                    trace("perform ${action.url}")
                    val answer = transport.perform(action.url, action.payload)
                    trace("answered ${answer::class.simpleName}")
                    dispatchNow(answer, controller, formId)
                }

                is UpdateSessionAction -> {
                    // The single point where the protocol touches authorisation (§12.4). The pair
                    // replaces whatever was held, and the screen behind it is the first one a
                    // signed-in person should see.
                    transport.useSession(action.accessToken)
                    loadGraph()
                    follow(DEFAULT_SCREEN)
                }

                is LoadPageAction -> {
                    // Pages are fetched by the toolkit's own list renderer; reaching one here means
                    // a button carried the action rather than a list, and re-opening the screen is
                    // the honest answer.
                    show(Screen.Failed("A page action arrived outside a list: ${action.url}"))
                }

                is CloseAction -> follow(DEFAULT_SCREEN)

                is CopyTextAction -> Unit

                else ->
                    // §2.1: an action of a type this build does not know is ignored, not fatal. The
                    // screen stays; the intent is dropped.
                    Unit
            }
        }
    }

    private suspend fun dispatchNow(
        action: KompotAction,
        controller: FormController,
        formId: String?,
    ) {
        when (action) {
            is UpdateSessionAction -> {
                transport.useSession(action.accessToken)
                loadGraph()
                follow(DEFAULT_SCREEN)
            }
            is NavigateAction -> follow(action.deeplink)
            else -> dispatch(action, controller, formId)
        }
    }

    /**
     * Resolves a deeplink through the graph.
     *
     * A deeplink the graph does not carry and this client does not know is **ignored** rather than
     * reported (§12.2). That is the rule, and it is also why the server has a test asserting every
     * deeplink it emits resolves somewhere: on this side the failure is silent by design — so the
     * one thing this can do is say so in the log, which is more than it used to.
     *
     * Public for the same reason [resolve] is: the destinations that do something other than open a
     * screen — signing out is one — are reachable only through here, and a test that cannot call it
     * can only assert where a deeplink points, not what following it undoes.
     */
    suspend fun follow(deeplink: String) {
        trace("follow $deeplink")
        when (val target = resolve(deeplink)) {
            is Target.Open -> open(target.path, target.kind)
            Target.Start -> start(door)
            Target.SignOut -> {
                transport.forgetSession()
                door?.close()
                start(door)
            }
            null -> println("tacku: nothing resolves \"$deeplink\"; the tap did nothing")
        }
    }

    /**
     * Where a deeplink leads, or null when nothing here knows.
     *
     * Separate from following it so that a test can ask the question without a server on the other
     * end — and there is a test, because this is the half where the failure is silent. The server
     * already checks that every deeplink it emits is either in the graph or on one of two lists it
     * keeps of what the client resolves natively. That check passed for a year while this function
     * had `else -> Unit` and no branch for the task prefix at all: the server's list said the client
     * knew it, and nobody asked the client. Opening a card did nothing, on every screen that has one.
     */
    fun resolve(deeplink: String): Target? {
        routes.firstOrNull { it.deeplink == deeplink }?.let {
            return Target.Open(it.endpoint, it.kind ?: "screen")
        }
        return when {
            deeplink == SIGN_IN -> Target.Open(SIGN_IN_PATH, "form")
            deeplink == SIGN_OUT -> Target.SignOut

            // The one destination the graph cannot carry: its endpoints are literal paths, so a
            // screen addressed by naming a thing is assembled here from a prefix and an identifier.
            else -> resolveTaskPath(deeplink)?.let { Target.Open(it, "form") }
        }
    }

    /** Where a deeplink leads. */
    sealed interface Target {
        data class Open(
            val path: String,
            val kind: String,
        ) : Target

        /** Forget the person, then begin as if the application had just opened. */
        data object SignOut : Target

        data object Start : Target
    }

    private suspend fun open(
        path: String,
        kind: String,
    ) {
        trace("open $path as $kind")
        when (kind) {
            "form" -> {
                val response = transport.form(path)
                show(Screen.Tree(response.screen, response.schema))
            }
            else -> show(Screen.Tree(transport.screen(path), null))
        }
    }

    suspend fun loadGraph() {
        routes = transport.graph().routes
    }

    /**
     * Which address a form submits to.
     *
     * Nothing on the wire ties a form identifier to an address: `submit_form` carries only the
     * identifier. So the server names each form after the address it submits to, and this is the
     * whole rule.
     *
     * It used to be a table of three exceptions, and the table is what the rule replaces. A form
     * whose identifier was not in it and did not match its route — the comment box on a task —
     * posted to an address that does not exist, and a POST into nothing looks exactly like a button
     * nobody pressed.
     */
    private fun submitPathFor(formId: String): String = "/submit/$formId"

    companion object {
        const val SIGN_IN = "app://sign-in"
        const val SIGN_OUT = "app://sign-out"
        const val SIGN_IN_PATH = "/forms/sign-in"
        const val DEFAULT_SCREEN = "app://catch-up"

        /**
         * Whether the client narrates what it did.
         *
         * A variable rather than an environment lookup, because reading the environment is a thing a
         * JVM can do and a page cannot. Set by whichever entry point knows how its platform is
         * configured; off by default, because the trace exists for an afternoon of debugging and not
         * for a product's log.
         */
        var traceEnabled: Boolean = false

        const val TASK_PREFIX = "app://task/"
        const val TASK_PATH = "/forms/task/"

        // Checked before the task prefix, and the order is the point: `app://edit-task/` does not
        // start with `app://task/`, but a prefix table is exactly where that stops being obvious.
        const val EDIT_TASK_PREFIX = "app://edit-task/"
        const val EDIT_TASK_PATH = "/forms/edit-task/"

        /**
         * The one address this client assembles instead of looking up.
         *
         * In the companion so that the probe can ask the same function the application asks. A probe
         * that spells the address itself keeps passing after the client stops resolving it — which
         * is exactly the failure this exists to catch.
         */
        fun resolveTaskPath(deeplink: String): String? =
            if (deeplink.startsWith(TASK_PREFIX) && deeplink.length > TASK_PREFIX.length) {
                TASK_PATH + deeplink.removePrefix(TASK_PREFIX)
            } else {
                null
            }
    }
}
