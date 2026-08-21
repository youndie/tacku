package tacku.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
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
import io.github.youndie.kompot.KompotRegistry
import io.github.youndie.kompot.KompotScreen
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import io.github.youndie.kompot.generated.generatedFormsClientRenderers
import io.github.youndie.kompot.kompotCoreRenderers
import io.github.youndie.kompot.kompotStandardRenderers
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

    // Core, standard and the form plug-in. Nothing of ours: a renderer written here would be this
    // client disagreeing with the toolkit about what a component looks like.
    val registry =
        remember {
            KompotRegistry(kompotCoreRenderers + kompotStandardRenderers + generatedFormsClientRenderers)
        }

    var screen by remember { mutableStateOf<Screen>(Screen.Loading) }
    val navigator = remember { Navigator(transport, scope) { screen = it } }

    LaunchedEffect(Unit) { navigator.start() }

    // The theme is Material 3 in the dark, and the token names the server sends resolve through the
    // design system rather than through anything here. A name it does not know costs a default and a
    // warning — never a broken screen (§6).
    MaterialTheme(colorScheme = darkColorScheme()) {
        // Ours, not the toolkit's Material default. The token names on the wire are this product's
        // — surface_block, agent, meta_agent — and a design system that does not know them resolves
        // every one to a default with a warning: a screen that renders, in the wrong colours, and
        // says so only in a log nobody is reading.
        CompositionLocalProvider(
            LocalKompotDesignSystem provides remember { TackuDesignSystem() },
            // Required rather than optional: the list renderer reads it and throws when it is
            // absent, so a screen with a list dies at render. Missing here until a screenshot of an
            // empty column said so.
            LocalKompotPageLoader provides remember(transport) { transport.pageLoader() },
        ) {
            when (val current = screen) {
                is Screen.Loading -> Message("Loading…")
                is Screen.Failed -> Message(current.reason)
                is Screen.Tree ->
                    Rendered(current.component, current.schema, registry, navigator, scope)
            }
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

    KompotScreen(
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
