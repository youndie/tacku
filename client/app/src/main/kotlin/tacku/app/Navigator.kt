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
    private val show: (Screen) -> Unit,
) {
    private var routes: List<ScreenRoute> = emptyList()

    /** The sign-in screen, which is the one place a client has to know without a graph. */
    fun start() {
        scope.launchCatching(onFailure = { show(Screen.Failed(it)) }) {
            open(SIGN_IN_PATH, kind = "form")
        }
    }

    fun handler(
        controller: FormController,
        formId: String?,
    ): KompotActionHandler = KompotActionHandler { action -> dispatch(action, controller, formId) }

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
                    val answer = transport.perform(action.url, action.payload)
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
     * deeplink it emits resolves somewhere: on this side the failure is silent by design.
     */
    private suspend fun follow(deeplink: String) {
        val route = routes.firstOrNull { it.deeplink == deeplink }
        if (route != null) {
            open(route.endpoint, route.kind ?: "screen")
            return
        }
        when (deeplink) {
            SIGN_IN -> open(SIGN_IN_PATH, kind = "form")
            SIGN_OUT -> start()
            else -> Unit
        }
    }

    private suspend fun open(
        path: String,
        kind: String,
    ) {
        when (kind) {
            "form" -> {
                val response = transport.form(path)
                show(Screen.Tree(response.screen, response.schema))
            }
            else -> show(Screen.Tree(transport.screen(path), null))
        }
    }

    private suspend fun loadGraph() {
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

    private companion object {
        const val SIGN_IN = "app://sign-in"
        const val SIGN_OUT = "app://sign-out"
        const val SIGN_IN_PATH = "/forms/sign-in"
        const val DEFAULT_SCREEN = "app://catch-up"
    }
}
