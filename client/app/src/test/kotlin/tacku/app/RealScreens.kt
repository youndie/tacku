package tacku.app

import androidx.compose.runtime.Composable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import ru.workinprogress.viddik.annotations.ViddikScreenshot
import java.io.File

/**
 * The screens themselves, photographed — which until today nobody had done.
 *
 * The five older shots are hand-written fragments: a pair of rows, an empty column, a refusal. They
 * are diagnostics, and they are worth having, but every one of them is a body somebody typed in
 * order to look at one thing. No picture of an actual screen existed, so a defect that lived on
 * every screen and on none of the fragments was invisible — which is exactly what happened: the
 * button drew as a Material pill in the running application for as long as the application had run.
 *
 * The bodies are frozen rather than fetched. A golden compared against a live server would be
 * comparing two things at once and going red for either, and half of what it caught would be a
 * clock. `make screens` regenerates them from a seeded server when the server's own output changes;
 * that the server still produces what these say is the Go half's job, and it has its own tests for
 * it.
 */
private val json = Json { ignoreUnknownKeys = true }

private fun screenOf(name: String): String {
    val file = File(System.getenv("TACKU_SCREEN_DIR") ?: "src/test/screens", "$name.json")
    val parsed = json.parseToJsonElement(file.readText()).jsonObject
    // A form endpoint answers a schema beside its tree; a screen endpoint answers the tree alone.
    return (parsed["screen"] ?: parsed).toString()
}

@ViddikScreenshot(name = "Board", group = "Screens", width = 1200, height = 900)
@Composable
fun RealBoard() = Shot(screenOf("board"))

@ViddikScreenshot(name = "Catch up", group = "Screens", width = 900, height = 900)
@Composable
fun RealCatchUp() = Shot(screenOf("catch-up"))

@ViddikScreenshot(name = "Task", group = "Screens", width = 900, height = 800)
@Composable
fun RealTask() = Shot(screenOf("task"))

@ViddikScreenshot(name = "New task", group = "Screens", width = 900, height = 780)
@Composable
fun RealNewTask() = Shot(screenOf("new-task"))

@ViddikScreenshot(name = "My tasks", group = "Screens", width = 900, height = 700)
@Composable
fun RealMyTasks() = Shot(screenOf("my-tasks"))

@ViddikScreenshot(name = "Sign in", group = "Screens", width = 900, height = 600)
@Composable
fun RealSignIn() = Shot(screenOf("sign-in"))
