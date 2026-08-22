package tacku.app

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.runComposeUiTest
import io.github.youndie.kompot.KompotRegistry
import io.github.youndie.kompot.KompotScreen
import io.github.youndie.kompot.LocalKompotDesignSystem
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.standard.EntityValue
import io.github.youndie.kompot.generated.generatedFormsClientRenderers
import io.github.youndie.kompot.kompotCoreRenderers
import io.github.youndie.kompot.kompotStandardRenderers
import io.github.youndie.kompot.standard.KompotPageLoader
import io.github.youndie.kompot.standard.KompotPageResponse
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * A filter is a control only if its value leaves the screen.
 *
 * The server has read `?status=` since the list was written, and nothing ever sent it: the list
 * carried no `reloadUrl`, so choosing a status changed a box on screen and nothing else. Nothing
 * failed — which is why this is a test of the toolkit's own behaviour rather than of a JSON body.
 * A body can declare the address correctly and the value can still never travel.
 */
class FilterWireTest {
    private val transport = Transport("http://localhost:0")

    private val registry =
        KompotRegistry(kompotCoreRenderers + kompotStandardRenderers + generatedFormsClientRenderers)

    // Compose asks for the main dispatcher and a plain JVM test has none. Set rather than worked
    // around: the composition has to run for the toolkit's own reload behaviour to be observed at
    // all, and that behaviour is the entire subject here.
    @BeforeTest
    fun setUp() = Dispatchers.setMain(UnconfinedTestDispatcher())

    @AfterTest
    fun tearDown() = Dispatchers.resetMain()

    @OptIn(ExperimentalTestApi::class)
    @Test
    fun `choosing a filter value asks the reload address for it`() =
        runComposeUiTest {
            val asked = mutableListOf<Pair<String, Map<String, String>>>()
            val loader =
                object : KompotPageLoader {
                    override suspend fun loadPage(
                        url: String,
                        query: Map<String, String>,
                    ): KompotPageResponse {
                        asked += url to query
                        return KompotPageResponse(items = emptyList(), nextLoadAction = null)
                    }
                }

            val response = transport.decodeForm(FILTERED_LIST)
            val controller =
                FormController(schema = response.schema, scope = CoroutineScope(Dispatchers.Unconfined))

            setContent {
                MaterialTheme(colorScheme = darkColorScheme()) {
                    CompositionLocalProvider(
                        LocalKompotDesignSystem provides TackuDesignSystem(),
                        LocalKompotPageLoader provides loader,
                    ) {
                        KompotScreen(
                            rootComponent = response.screen,
                            registry = registry,
                            formController = controller,
                            actionHandler = { },
                        )
                    }
                }
            }
            waitForIdle()

            // An entity and not text: a selection sends what it selected, and putting text here
            // throws in the renderer. That mismatch was live on the server side too.
            controller.onValueChanged("status", EntityValue(id = "in_progress", title = "In progress"))
            // Waited on rather than asserted immediately: a reload may well be debounced, and a check
            // that looks once would call a slow wire a missing one.
            runCatching { waitUntil(timeoutMillis = 3_000) { asked.isNotEmpty() } }

            assertTrue(
                asked.any { (url, query) -> url == "/pages/tasks" && query["status"] == "in_progress" },
                "choosing a status asked for $asked — the value never left the screen, so the filter is a control in appearance only",
            )
        }

    private companion object {
        // The shape the server sends: a select declared in the schema, and a list that reloads from
        // an address. Written out rather than fetched, so the test says which half it is checking.
        val FILTERED_LIST = """
            {
              "schema": {
                "formId": "my-tasks",
                "fields": [
                  {"type":"selection_field","fieldId":"status","initialValue":null,
                   "options":[{"id":"in_progress","label":"In progress"}],"validationRules":[]}
                ]
              },
              "screen": {
                "type":"column","id":"screen-my-tasks","children":[
                  {"type":"select_input","id":"field-status","fieldId":"status",
                   "label":"Status","placeholder":"Any status",
                   "options":[{"id":"in_progress","label":"In progress"}]},
                  {"type":"paginated_list","id":"my-tasks-list","initialItems":[],
                   "reloadUrl":"/pages/tasks"}
                ]
              }
            }
        """
    }
}
