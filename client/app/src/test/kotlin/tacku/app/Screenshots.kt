package tacku.app

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import io.github.youndie.kompot.KompotLazyScreen
import io.github.youndie.kompot.LocalKompotPageLoader
import io.github.youndie.kompot.form.FormController
import io.github.youndie.kompot.form.FormSchema
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import ru.workinprogress.viddik.annotations.ViddikScreenshot
import ru.workinprogress.viddik.core.ViddikFontFamily
import ru.workinprogress.viddik.core.ViddikPlatformTextStyle
import ru.workinprogress.viddik.core.viddikTypography

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
internal val transport = Transport("http://localhost:0")

private val registry = tackuRegistry()

/**
 * The font is carried in, not found on the machine.
 *
 * A [TextStyle] with no family is drawn in whatever the host has installed, and the same five
 * screens recorded on two operating systems then differ in glyphs — measured here at 2.5-8.6% of
 * pixels before the font was pinned.
 *
 * **The harness's font, not the product's, and that was settled by measurement rather than taste.**
 * Pinning IBM Plex Sans — the typeface the design is drawn in, which the product now carries itself —
 * made twelve of thirteen screens disagree between a mac and a Linux runner, by 0.09% to 3.27% of
 * pixels, while the one screen with no text came out identical. Normalising its vertical metrics with
 * viddik's own tool changed nothing: the same twelve, the same numbers. So the cause is the
 * rasterisation of that file and not the table its metrics live in, and the font viddik bundles for
 * exactly this purpose is the only one that comes out the same on both machines.
 *
 * What that costs is [ScreenTextCoverageTest]: the bundled font does not carry every character this
 * product draws, and one it lacks moves a button two pixels. The test names the ones we know about,
 * so the next uncovered character fails as a sentence instead of as a pixel count.
 *
 * The platform style stays: it is what keeps line metrics identical across machines.
 */
private val viddikBase =
    TextStyle(
        fontFamily = ViddikFontFamily,
        platformStyle = ViddikPlatformTextStyle,
    )

private val design = TackuDesignSystem(base = viddikBase)

/**
 * The product's three faces, with the metrics both Skia backends will agree on.
 *
 * Read from the same jar the desktop reads them from, so there is one copy of the files and the
 * picture cannot drift from what ships.
 */
@Composable
internal fun Shot(body: String) {
    TackuTheme(design, typography = viddikTypography(Typography())) {
        Inner(body)
    }
}

@Composable
internal fun Inner(body: String) {
    CompositionLocalProvider(
        LocalKompotPageLoader provides transport.pageLoader(),
    ) {
        // White, exactly like the window of the running application, and not the product's surface.
        //
        // This used to paint `#101114` — the surface token, written out by hand — behind every
        // screen, so a screen that did not cover the window looked identical to one that did. Two
        // forms did not: their root put `padding` before `background`, so the fill stopped 32dp
        // short on every side and the application showed a white frame. The harness had been
        // hiding it since the first golden.
        Box(Modifier.fillMaxSize().background(Color.White)) {
            KompotLazyScreen(
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
 *
 * It was not enough. For the whole life of this project the stripe painted nothing — an empty column
 * has no height — and this picture passed anyway, because the two rows stayed distinguishable by the
 * colour of their meta line. "Distinguishable" was the wrong thing to check: the question is whether
 * the *device the design chose* is there, and a picture where it is missing and something else
 * happens to differ answers yes.
 */
@ViddikScreenshot(name = "Provenance — agent beside a person", group = "States", width = 520, height = 200)
@Composable
fun ProvenancePair() =
    Shot(
        """
        {"type":"column","id":"pair","spacing":8,"children":[
          {"type":"row","id":"r1","children":[
            {"type":"column","id":"r1s","modifiers":[{"type":"size","widthDp":3,"height":"Fill"},{"type":"background","color":"agent"}],"children":[]},
            {"type":"column","id":"r1b","spacing":6,"modifiers":[{"type":"weight","value":1},{"type":"padding","all":16},{"type":"background","color":"surface_block"}],"children":[
              {"type":"text","id":"r1t","text":"Moved “Fix login redirect loop” from In progress to In review","style":"body"},
              {"type":"text","id":"r1a","text":"Agent · on behalf of Anna Petrova · 04:12","style":"meta_agent"}]}]},
          {"type":"row","id":"r2","children":[
            {"type":"column","id":"r2s","modifiers":[{"type":"size","widthDp":3,"height":"Fill"},{"type":"background","color":"divider"}],"children":[]},
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
          {"type":"column","id":"refs","modifiers":[{"type":"size","widthDp":3,"height":"Fill"},{"type":"background","color":"agent"}],"children":[]},
          {"type":"column","id":"refb","spacing":8,"modifiers":[{"type":"weight","value":1},{"type":"padding","all":16},{"type":"background","color":"danger"}],"children":[
            {"type":"text","id":"reft","text":"This task was closed by Ivan Sokolov 4 minutes ago, so your change was not applied.","style":"body"},
            {"type":"text","id":"refa","text":"Your agent tried the same change at 04:12 and was refused too.","style":"meta_agent"},
            {"type":"row","id":"refr","children":[
              {"type":"button","id":"refbtn","text":"Reload the task","action":{"type":"navigate","deeplink":"app://board"}},
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

/**
 * Two accents that have to be the same accent.
 *
 * The left block is painted by the server: a `background` modifier naming `accent`, resolved through
 * the design system. The button beside it is painted by nobody here — the toolkit draws it, and it
 * takes its colour from `MaterialTheme`. Those are two different routes to a colour, and for most of
 * this project's life they led to two different colours: the running application showed this
 * deployment's blue behind a Material-default lavender button, and every test agreed, because the
 * harness had the same baseline theme the application did.
 *
 * So this is a comparison, not a picture of a button. If the theme is ever built from something
 * other than the tokens, the two halves stop matching and the picture says so. The corners are the
 * second half of the same question: the vocabulary has no radius modifier, so a control that rounds
 * itself is the client answering a question the protocol does not let anybody ask.
 *
 * The third button is here because emphasis has nowhere else to live. `button` carries no variant,
 * so a fill is the only thing separating the action from the way out, and if the two ever draw
 * alike the screen has one button twice.
 */
@ViddikScreenshot(name = "Does a control take the product accent", group = "Diagnostics", width = 520, height = 140)
@Composable
fun ControlAccent() =
    Shot(
        """
        {"type":"row","id":"ca","spacing":16,"modifiers":[{"type":"padding","all":16},{"type":"background","color":"surface_block"}],"children":[
          {"type":"column","id":"painted","modifiers":[{"type":"size","widthDp":120,"heightDp":40},{"type":"background","color":"accent"}],"children":[]},
          {"type":"button","id":"drawn","text":"Sign in","modifiers":[{"type":"padding","top":14,"bottom":14,"start":24,"end":24},{"type":"background","color":"accent"}],"action":{"type":"navigate","deeplink":"app://board"}},
          {"type":"button","id":"quiet","text":"Cancel","modifiers":[{"type":"padding","top":14,"bottom":14,"start":24,"end":24}],"action":{"type":"navigate","deeplink":"app://board"}},
          {"type":"column","id":"casp","modifiers":[{"type":"weight","value":1}],"children":[]}]}
        """.trimIndent(),
    )

/**
 * A three-point stripe that has to be exactly as tall as the card beside it.
 *
 * The stripe is how this product says an agent did something, and it is an empty column, because the
 * vocabulary has no border. An empty column has no height, so it painted nothing — on every card of
 * every screen, for the whole life of the project, while the picture guarding it passed because the
 * two rows stayed distinguishable by their meta line.
 *
 * `height: Fill` does not fix it: `Fill` resolves against the constraint coming *into* the row, not
 * against the height of the sibling, so the stripe takes the whole screen and drags the row with it;
 * `Wrap` on the row does not change that, and an explicit `heightDp` would be a guess at how tall
 * the text wrapped.
 *
 * The second card here is the way out, and it needs no new modifier: paint the *outer* node with the
 * stripe colour, inset it three points from the start, and let the inner node paint everything else.
 * The stripe is then whatever the card's height turns out to be, because it is the card.
 */
@ViddikScreenshot(name = "A stripe as tall as its card", group = "Diagnostics", width = 520, height = 340)
@Composable
fun AStripeAsTallAsItsCard() =
    Shot(
        """
        {"type":"column","id":"sv","spacing":10,"modifiers":[{"type":"size","width":"Fill"},{"type":"padding","all":10},{"type":"background","color":"surface"}],"children":[
          {"type":"text","id":"sv0-l","text":"sibling column, widthDp 3","style":"label"},
          {"type":"row","id":"sa","modifiers":[{"type":"size","width":"Fill"}],"children":[{"type":"column","id":"sa-s","modifiers":[{"type":"size","widthDp":3},{"type":"background","color":"agent"}],"children":[]},{"type":"column","id":"sa-b","modifiers":[{"type":"background","color":"surface_field"},{"type":"size","width":"Fill"},{"type":"weight","value":1},{"type":"padding","all":10}],"children":[{"type":"text","id":"sa-t","text":"the stripe as a sibling: nothing is painted","style":"body"}]}]},
          {"type":"text","id":"sv1-l","text":"outer background, padding start 3","style":"label"},
          {"type":"column","id":"se","modifiers":[{"type":"background","color":"agent"},{"type":"size","width":"Fill"},{"type":"padding","start":3}],"children":[{"type":"column","id":"se-b","modifiers":[{"type":"background","color":"surface_field"},{"type":"size","width":"Fill"},{"type":"padding","all":10}],"children":[{"type":"text","id":"se-t","text":"two lines of text so the card has some height at all, and a little more so that it wraps onto a second line","style":"body"}]}]}]}
        """.trimIndent(),
    )
