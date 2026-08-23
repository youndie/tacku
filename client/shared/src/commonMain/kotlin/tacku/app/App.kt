package tacku.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotLazyScreen
import io.github.youndie.kompot.KompotRealtimeProvider
import io.github.youndie.kompot.KompotRegistry
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.TypographyToken
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

@Composable
fun App(baseUrl: String) {
    val scope = rememberCoroutineScope()

    // One door, not one per caller: it holds the verifier of a sign-in that is in flight, and two
    // would be two halves of one exchange.
    val door = remember(baseUrl) { platformDoor(baseUrl) }
    val transport = remember(baseUrl) { Transport(baseUrl) }
    val updates = remember(baseUrl) { Updates(baseUrl) { transport.accessToken } }

    // Core, standard, the form plug-in and this deployment's own — the same set the screenshots
    // draw, which is the only way a golden is a picture of the product.
    val registry = remember { tackuRegistry() }

    var screen by remember { mutableStateOf<Screen>(Screen.Loading) }
    val navigator =
        remember {
            // The trace is the navigator's own, and it is one line rather than a flag read here:
            // reading the environment is a thing a JVM can do and a page cannot.
            Navigator(transport, scope, door) { state -> screen = state }
        }

    LaunchedEffect(Unit) { navigator.start(door) }

    // The theme is Material 3 in the dark, and the token names the server sends resolve through the
    // design system rather than through anything here. A name it does not know costs a default and a
    // warning — never a broken screen (§6).
    // Material's own colours and shapes come from the same tokens the server names, and that is the
    // point rather than a nicety: a control the toolkit draws for itself would otherwise use the
    // baseline scheme, so the product would have two palettes and only one of them its own.
    // The family before the first frame, not after it. In a page the three files arrive over the
    // network, and drawing once in the machine's font and again in the product's is two pictures
    // where there should be one — the difference nobody sees because both look like a screen.
    val family by produceState<FontFamily?>(initialValue = null) { value = loadTackuFontFamily() }
    val design = remember(family) { family?.let { TackuDesignSystem(base = TextStyle(fontFamily = it)) } }

    if (design == null) {
        return
    }

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
    // The product's own body style, not Material's: a message drawn in a colour nobody chose is how
    // "the server is unreachable" became a blank white window with an invisible line on it.
    val design = LocalKompotDesignSystem.current

    Box(Modifier.fillMaxSize().padding(32.dp)) {
        Text(text, style = design.resolveTypography(TypographyToken("body")))
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
