package tacku.tck

import io.github.youndie.kompot.tck.TckReport
import io.github.youndie.kompot.tck.TckSkip
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

/**
 * What it means for a conformance run to have proved something.
 *
 * The kit's own [TckReport.isClean] answers "no findings", and that is not the same question. A
 * check that finds no endpoint to apply to passes in silence, so a server missing half its endpoint
 * kinds produces a clean report that demonstrates nothing — the failure mode this project is most
 * exposed to, because the acceptance half of it is the whole point.
 *
 * The gate therefore asks two things at once: nothing was found wrong, **and** every check had
 * something to look at.
 */
object TckGate {
    /**
     * The eight checks the kit runs, by the names it reports them under.
     *
     * Hardcoded because the kit exposes no list of them, and read back out of the published
     * artefact rather than out of its README so that the names are the ones actually emitted.
     */
    val expectedChecks: Set<String> =
        setOf(
            "schema",
            "auth-required",
            "component-id",
            "form-fields",
            "etag",
            "pagination",
            "navigation",
            "idempotency",
            // Ninth, arriving with kompot 0.11 and the action that acts on one item of a list. The
            // guard below found it rather than a changelog: the run reported a check this list did
            // not hold, which is the whole reason the list is checked in both directions.
            "perform",
        )

    /**
     * The verdict, in the two directions that matter.
     *
     * [unexercised] is the point of the gate. [unknown] is its guard: if the kit gains a ninth
     * check, a gate that only looked for its eight would keep passing while covering less than it
     * claims — a silent narrowing, which is worse than a loud break.
     */
    data class Verdict(
        val unexercised: List<String>,
        val unknown: List<String>,
        val findings: Int,
        val exercised: Map<String, Int>,
        val bodiesDeclared: Int = 0,
        val bodiesChecked: Int = 0,
        /** What the walk decided not to fetch, and why — in the kit's own words, as of 0.15. */
        val skipped: List<TckSkip> = emptyList(),
    ) {
        val passed: Boolean
            get() =
                unexercised.isEmpty() &&
                    unknown.isEmpty() &&
                    findings == 0 &&
                    bodiesChecked >= bodiesDeclared
    }

    /**
     * Endpoints whose successful body the `schema` check validates.
     *
     * Counting them is the second half of the same argument the target counters make. A check with
     * no targets proves nothing; so does a check with plenty of targets that happens to have skipped
     * a whole endpoint. The first is visible in the report, the second is not — the run that
     * prompted this was green while the most complicated screen in the product had never been
     * fetched, because its path carries a parameter and the walk quietly passed over it.
     *
     * `wizard_start` is here because it answers a document like the other four — a
     * `KompotFormResponse` (§16.1) — and leaving it out would have meant a new endpoint kind
     * arriving without the count noticing. Its neighbour `wizard_resume` is not: the walk is blind
     * and only fetches, so a transition that has to carry the scenario a previous answer minted is
     * out of its reach by construction, and the report says so by name.
     */
    private val bodyKinds = setOf("screen", "form", "page", "graph", "wizard_start")

    fun judge(
        report: TckReport,
        openApi: JsonObject? = null,
    ): Verdict {
        val exercised = report.exercised
        val declared = openApi?.let { countBodyEndpoints(it) } ?: 0
        return Verdict(
            unexercised = expectedChecks.filter { (exercised[it] ?: 0) < 1 }.sorted(),
            unknown = exercised.keys.filterNot { it in expectedChecks }.sorted(),
            findings = report.findings.size,
            exercised = exercised.toSortedMap(),
            bodiesDeclared = declared,
            bodiesChecked = exercised["schema"] ?: 0,
            skipped = report.skipped,
        )
    }

    private fun countBodyEndpoints(openApi: JsonObject): Int {
        val paths = openApi["paths"] as? JsonObject ?: return 0
        return paths.values.sumOf { path ->
            (path as? JsonObject)
                ?.values
                ?.count { operation ->
                    val kind =
                        (
                            (operation as? JsonObject)
                                ?.get("x-kompot-endpoint-kind") as? JsonPrimitive
                        )?.content
                    kind in bodyKinds
                } ?: 0
        }
    }

    /**
     * Throws with a report a person can act on. Called from the test that runs the kit.
     */
    fun require(
        report: TckReport,
        openApi: JsonObject? = null,
    ) {
        val verdict = judge(report, openApi)
        if (!verdict.passed) {
            throw AssertionError(describe(report, verdict))
        }
    }

    /**
     * The counters are printed on a green run too, and deliberately: a number is the only thing
     * that distinguishes a check that looked and found nothing wrong from one that never looked.
     */
    fun describe(
        report: TckReport,
        verdict: Verdict = judge(report),
    ): String =
        buildString {
            appendLine(if (verdict.passed) "TCK: passed" else "TCK: failed")
            appendLine()
            appendLine("targets per check:")
            expectedChecks.sorted().forEach { check ->
                val count = verdict.exercised[check] ?: 0
                val note =
                    if (count < 1) {
                        "0  <- no target: this check proved nothing"
                    } else {
                        count.toString()
                    }
                appendLine("  %-14s %s".format(check, note))
            }

            if (verdict.bodiesDeclared > 0) {
                appendLine()
                appendLine(
                    "bodies: %d of %d declared endpoints had a response checked".format(
                        verdict.bodiesChecked,
                        verdict.bodiesDeclared,
                    ),
                )
                if (verdict.bodiesChecked < verdict.bodiesDeclared) {
                    appendLine(
                        "  an endpoint the walk never fetched is one no check could have failed, " +
                            "and a report cannot say so about an endpoint it skipped",
                    )
                }
            }

            // Outside the body count on purpose, and a test says so. The kit reports a skip whether
            // or not this gate was handed an OpenAPI description to count endpoints in, and nesting
            // the two together meant a walk that skipped something said nothing about it in every
            // run that did not also count bodies.
            //
            // Named rather than counted, which the kit could not do until 0.15. A number told us
            // one endpoint was missing and left the reader to work out which — and it was hiding
            // five, three of them holes in our own configuration.
            if (verdict.skipped.isNotEmpty()) {
                appendLine()
                verdict.skipped.forEach { skip ->
                    appendLine("  skipped: %s %s — %s".format(skip.method, skip.path, skip.reason))
                }
            }

            if (verdict.unknown.isNotEmpty()) {
                appendLine()
                appendLine("checks reported that this gate does not know about: ${verdict.unknown.joinToString()}")
                appendLine(
                    "  the kit has grown; add them to TckGate.expectedChecks or the gate covers less than it claims",
                )
            }

            if (report.findings.isNotEmpty()) {
                appendLine()
                appendLine("findings (${report.findings.size}):")
                report.findings.forEach { appendLine("  [${it.check}] ${it.target}: ${it.message}") }
            }
        }
}
