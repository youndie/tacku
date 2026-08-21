package tacku.app

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
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
import kotlinx.coroutines.Dispatchers
import ru.workinprogress.viddik.annotations.ViddikScreenshot

/**
 * What the states actually look like.
 *
 * The subjects are chosen the way the backlog item asked: the states before the screens. A screen
 * that renders tells you nothing about which branch it took, and the branches nobody exercises by
 * hand are exactly the ones that rot — an unknown component, an empty column, a field in error.
 *
 * Each subject is a **JSON body**, not a hand-built tree. That keeps the picture honest about the
 * path it went down: the toolkit decodes and the toolkit draws, so the screenshot is of the same
 * machinery the product uses rather than of a Compose function written to look like it.
 */
private val transport = Transport("http://localhost:0")

private val registry =
    KompotRegistry(kompotCoreRenderers + kompotStandardRenderers + generatedFormsClientRenderers)

@Composable
private fun Shot(body: String) {
    MaterialTheme(colorScheme = darkColorScheme()) {
        Inner(body)
    }
}

@Composable
private fun Inner(body: String) {
    CompositionLocalProvider(
        LocalKompotDesignSystem provides TackuDesignSystem(),
        LocalKompotPageLoader provides transport.pageLoader(),
    ) {
        Box(Modifier.fillMaxSize().background(Color(0xFF101114)).padding(16.dp)) {
            KompotScreen(
                rootComponent = transport.decodeScreen(body),
                registry = registry,
                formController =
                    FormController(
                        schema = FormSchema(formId = "shot", fields = emptyList()),
                        scope = CoroutineScope(Dispatchers.Unconfined),
                    ),
                actionHandler = { },
            )
        }
    }
}

/**
 * The pair the product exists for, side by side.
 *
 * If the two rows ever stop being distinguishable, the promise the tracker makes stops being kept —
 * and it would stop quietly, which is why this one is a picture rather than an assertion.
 */
@ViddikScreenshot(name = "Provenance — agent beside a person", group = "States", width = 520, height = 200)
@Composable
fun ProvenancePair() =
    Shot(
        """
        {"type":"column","id":"pair","spacing":8,"children":[
          {"type":"row","id":"r1","children":[
            {"type":"column","id":"r1s","modifiers":[{"type":"size","widthDp":3},{"type":"background","color":"agent"}],"children":[]},
            {"type":"column","id":"r1b","spacing":6,"modifiers":[{"type":"weight","value":1},{"type":"padding","all":16},{"type":"background","color":"surface_block"}],"children":[
              {"type":"text","id":"r1t","text":"Moved “Fix login redirect loop” from In progress to In review","style":"body"},
              {"type":"text","id":"r1a","text":"Agent · on behalf of Anna Petrova · 04:12","style":"meta_agent"}]}]},
          {"type":"row","id":"r2","children":[
            {"type":"column","id":"r2s","modifiers":[{"type":"size","widthDp":3},{"type":"background","color":"divider"}],"children":[]},
            {"type":"column","id":"r2b","spacing":6,"modifiers":[{"type":"weight","value":1},{"type":"padding","all":16},{"type":"background","color":"surface_block"}],"children":[
              {"type":"text","id":"r2t","text":"Commented on “Fix login redirect loop”","style":"body"},
              {"type":"text","id":"r2a","text":"Ivan Sokolov · 09:31","style":"meta"}]}]}]}
        """.trimIndent(),
    )

/**
 * What a client shows for a node from a newer server.
 *
 * Users of an older build see this more often than anybody would like, and it is the one picture
 * whose tone matters more than its layout: "something new is here", not "something broke".
 */
@ViddikScreenshot(name = "Unknown component", group = "States", width = 520, height = 220)
@Composable
fun UnknownComponent() =
    Shot(
        """
        {"type":"column","id":"u","spacing":8,"children":[
          {"type":"text","id":"before","text":"The rest of this screen works as usual.","style":"body"},
          {"type":"timeline","id":"future","events":[]},
          {"type":"text","id":"after","text":"And so does this.","style":"body_muted"}]}
        """.trimIndent(),
    )

/**
 * A column with nothing in it: one line and no call to action, because four empty columns on a
 * Monday morning would otherwise be four invitations on one screen.
 */
@ViddikScreenshot(name = "Empty column", group = "States", width = 320, height = 200)
@Composable
fun EmptyColumn() =
    Shot(
        """
        {"type":"column","id":"col","spacing":12,"modifiers":[{"type":"padding","all":12},{"type":"background","color":"surface_block"}],"children":[
          {"type":"row","id":"head","children":[
            {"type":"text","id":"name","text":"DONE","style":"subtitle"},
            {"type":"column","id":"sp","modifiers":[{"type":"weight","value":1}],"children":[]},
            {"type":"text","id":"count","text":"0","style":"meta"}]},
          {"type":"paginated_list","id":"list","initialItems":[],
           "emptyState":{"type":"text","id":"empty","text":"Nothing finished yet this sprint.","style":"body_muted"}}]}
        """.trimIndent(),
    )

/** The refusal that is not a field error: an outcome, with a cause, an actor and one way out. */
@ViddikScreenshot(name = "Refused on the merits", group = "States", width = 520, height = 220)
@Composable
fun Refusal() =
    Shot(
        """
        {"type":"row","id":"ref","children":[
          {"type":"column","id":"refs","modifiers":[{"type":"size","widthDp":3},{"type":"background","color":"agent"}],"children":[]},
          {"type":"column","id":"refb","spacing":8,"modifiers":[{"type":"weight","value":1},{"type":"padding","all":16},{"type":"background","color":"danger"}],"children":[
            {"type":"text","id":"reft","text":"This task was closed by Ivan Sokolov 4 minutes ago, so your change was not applied.","style":"body"},
            {"type":"text","id":"refa","text":"Your agent tried the same change at 04:12 and was refused too.","style":"meta_agent"},
            {"type":"row","id":"refr","children":[
              {"type":"button","id":"refbtn","text":"Reload the task","modifiers":[{"type":"padding","top":10,"bottom":10,"start":18,"end":18},{"type":"background","color":"accent"}],"action":{"type":"navigate","deeplink":"app://board"}},
              {"type":"column","id":"refsp","modifiers":[{"type":"weight","value":1}],"children":[]}]}]}]}
        """.trimIndent(),
    )

/**
 * Deliberately a comparison rather than a picture of something.
 *
 * The design's first rule is that a typography token carries size, weight **and** colour, because
 * `text` has no colour field at all. If the two lines below come out the same colour, that rule does
 * not hold, and every red error message and every amber agent line in the design means nothing.
 */
@ViddikScreenshot(name = "Does a token carry colour", group = "Diagnostics", width = 520, height = 160)
@Composable
fun TokenColour() =
    Shot(
        """
        {"type":"column","id":"tc","spacing":8,"modifiers":[{"type":"padding","all":16},{"type":"background","color":"surface_block"}],"children":[
          {"type":"text","id":"a","text":"body — should be near-white","style":"body"},
          {"type":"text","id":"b","text":"error — should be red","style":"error"},
          {"type":"text","id":"c","text":"meta_agent — should be purple","style":"meta_agent"},
          {"type":"text","id":"d","text":"no style at all","style":null}]}
        """.trimIndent(),
    )
