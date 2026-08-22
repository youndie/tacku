package tacku.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotLazyScreen
import io.github.youndie.kompot.KompotRealtimeProvider
import io.github.youndie.kompot.KompotRegistry
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/**
 * The desktop client.
 *
 * Almost none of the rendering is here: the toolkit draws a tree from the wire, and what this
 * application supplies is a window, a transport, a design system and the handling of intents. That
 * ratio is the point — a screen ships without touching this file, and the day it does not, something
 * has gone wrong upstream.
 */
fun main() =
    application {
        val state = rememberWindowState(width = 1440.dp, height = 900.dp)
        Window(onCloseRequest = ::exitApplication, title = "tacku", state = state) {
            App(baseUrl = System.getenv("TACKU_URL") ?: "http://localhost:8477")
        }
    }

@Composable
private fun App(baseUrl: String) {
    val scope = rememberCoroutineScope()
    val transport = remember(baseUrl) { Transport(baseUrl) }
    val updates = remember(baseUrl) { Updates(baseUrl) { transport.accessToken } }

    // Core, standard, the form plug-in and this deployment's own — the same set the screenshots
    // draw, which is the only way a golden is a picture of the product.
    val registry = remember { tackuRegistry() }

    var screen by remember { mutableStateOf<Screen>(Screen.Loading) }
    val navigator = remember { Navigator(transport, scope) { screen = it } }

    LaunchedEffect(Unit) { navigator.start() }

    // The theme is Material 3 in the dark, and the token names the server sends resolve through the
    // design system rather than through anything here. A name it does not know costs a default and a
    // warning — never a broken screen (§6).
    // Material's own colours and shapes come from the same tokens the server names, and that is the
    // point rather than a nicety: a control the toolkit draws for itself would otherwise use the
    // baseline scheme, so the product would have two palettes and only one of them its own.
    val design = rememberTackuDesignSystem()

    // Colours, shapes and the colour a control uses when it names none — the same wrapper the
    // screenshots draw through, because two copies of this is how the pictures stopped being of
    // the product.
    TackuTheme(design) {
        CompositionLocalProvider(
            LocalKompotPageLoader provides remember(transport) { transport.pageLoader() },
        ) {
            // Everything drawn sits inside the live channel rather than the board alone: the
            // provider holds the frames that have arrived and the toolkit replaces a node by its
            // identifier when it draws, so a screen that never sees a frame takes exactly the same
            // path as one that does.
            //
            // The topic is a name for the subscription, not a request. This client's source ignores
            // it and asks for whatever the token it holds entitles it to, which is the server's
            // topic rule seen from the other end.
            //
            // `content` is passed by name because it is not the last parameter: a trailing lambda
            // here binds to the error handler that follows it, and the compiler says only that
            // `content` is missing. The screen would have been drawn as an error callback.
            KompotRealtimeProvider(
                topic = "self",
                source = updates,
                content = {
                    when (val current = screen) {
                        is Screen.Loading -> Message("Loading…")
                        is Screen.Failed -> Message(current.reason)
                        is Screen.Tree ->
                            Rendered(current.component, current.schema, registry, navigator, scope)
                    }
                },
            )
        }
    }
}

@Composable
private fun Rendered(
    component: KompotComponent,
    schema: FormSchema?,
    registry: KompotRegistry,
    navigator: Navigator,
    scope: CoroutineScope,
) {
    // A form controller belongs to one form: it holds the values, the visibility and the patches.
    // Keyed by the identifier so that moving between screens does not carry one form's answers into
    // another.
    val controller =
        remember(schema?.formId) {
            FormController(
                schema = schema ?: FormSchema(formId = "", fields = emptyList()),
                scope = scope,
            )
        }

    KompotLazyScreen(
        rootComponent = component,
        registry = registry,
        formController = controller,
        actionHandler = remember(controller) { navigator.handler(controller, schema?.formId) },
    )
}

@Composable
private fun Message(text: String) {
    Box(Modifier.fillMaxSize().padding(32.dp)) {
        Text(text, style = MaterialTheme.typography.bodyLarge)
    }
}

/** What the window is showing. */
sealed interface Screen {
    data object Loading : Screen

    data class Failed(
        val reason: String,
    ) : Screen

    /** A tree, and the schema beside it when the tree came from a form. */
    data class Tree(
        val component: KompotComponent,
        val schema: FormSchema?,
    ) : Screen
}

internal fun CoroutineScope.launchCatching(
    onFailure: (String) -> Unit,
    block: suspend () -> Unit,
) {
    launch {
        runCatching { block() }
            .onFailure { failure ->
                onFailure(
                    when (failure) {
                        is ServerRefused -> "${failure.status}: ${failure.reason}"
                        else -> failure.message ?: failure::class.simpleName.orEmpty()
                    },
                )
            }
    }
}
