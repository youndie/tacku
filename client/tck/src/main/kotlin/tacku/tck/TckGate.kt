package tacku.tck

import io.github.youndie.kompot.tck.TckReport

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
    ) {
        val passed: Boolean get() = unexercised.isEmpty() && unknown.isEmpty() && findings == 0
    }

    fun judge(report: TckReport): Verdict {
        val exercised = report.exercised
        return Verdict(
            unexercised = expectedChecks.filter { (exercised[it] ?: 0) < 1 }.sorted(),
            unknown = exercised.keys.filterNot { it in expectedChecks }.sorted(),
            findings = report.findings.size,
            exercised = exercised.toSortedMap(),
        )
    }

    /**
     * Throws with a report a person can act on. Called from the test that runs the kit.
     */
    fun require(report: TckReport) {
        val verdict = judge(report)
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
