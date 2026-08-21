package tacku.tck

import io.github.youndie.kompot.tck.TckFinding
import io.github.youndie.kompot.tck.TckReport
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * The gate is tested against reports rather than against a server, and that is not a compromise:
 * a report is its entire input. Wiring it to a live server is [B-14], and by then the thing doing
 * the judging will already have been shown to judge correctly.
 */
class TckGateTest {
    private fun report(
        exercised: Map<String, Int>,
        findings: List<TckFinding> = emptyList(),
    ) = TckReport(findings, exercised, emptySet())

    private val everyCheckBusy = TckGate.expectedChecks.associateWith { 3 }

    @Test
    fun `a run where every check found a target and nothing was wrong passes`() {
        val verdict = TckGate.judge(report(everyCheckBusy))

        assertTrue(verdict.passed, TckGate.describe(report(everyCheckBusy)))
        assertEquals(emptyList(), verdict.unexercised)
    }

    /**
     * The case the gate exists for. Without it this report is indistinguishable from success: the
     * kit found nothing wrong, because it had nothing to look at.
     */
    @Test
    fun `a check with no target fails the run even though nothing was found wrong`() {
        val missingPagination = everyCheckBusy - "pagination"
        val subject = report(missingPagination)

        assertTrue(subject.isClean, "the kit's own verdict is clean, which is exactly the problem")

        val verdict = TckGate.judge(subject)
        assertFalse(verdict.passed)
        assertEquals(listOf("pagination"), verdict.unexercised)
        assertTrue(
            TckGate.describe(subject).contains("this check proved nothing"),
            "the report must say which check had nothing to look at, not merely that the run failed",
        )
    }

    @Test
    fun `a check present but counted zero is the same failure`() {
        val verdict = TckGate.judge(report(everyCheckBusy + ("etag" to 0)))

        assertFalse(verdict.passed)
        assertEquals(listOf("etag"), verdict.unexercised)
    }

    /**
     * The guard in the other direction. A kit that grows a ninth check would otherwise be covered
     * by a gate that keeps passing while checking less than it claims — a silent narrowing, which
     * is worse than a loud break.
     */
    @Test
    fun `a check the gate does not know about fails loudly`() {
        val subject = report(everyCheckBusy + ("realtime-frames" to 4))
        val verdict = TckGate.judge(subject)

        assertFalse(verdict.passed, "an unknown check was ignored instead of reported")
        assertEquals(listOf("realtime-frames"), verdict.unknown)
        assertTrue(TckGate.describe(subject).contains("the kit has grown"))
    }

    @Test
    fun `findings still fail the run`() {
        val subject = report(everyCheckBusy, listOf(TckFinding("etag", "/screens/board", "no 304 on If-None-Match")))
        val verdict = TckGate.judge(subject)

        assertFalse(verdict.passed)
        assertEquals(1, verdict.findings)
        assertTrue(TckGate.describe(subject).contains("no 304 on If-None-Match"))
    }

    /**
     * A green run prints its counters too. The number is the only thing separating a check that
     * looked and found nothing from one that never looked, so hiding it on success would remove
     * the evidence exactly when nobody is inclined to ask for it.
     */
    @Test
    fun `a passing run still reports how many targets each check had`() {
        val text = TckGate.describe(report(everyCheckBusy))

        assertTrue(text.contains("TCK: passed"))
        TckGate.expectedChecks.forEach { check ->
            assertTrue(text.contains(check), "the passing report does not mention $check")
        }
    }

    @Test
    fun `require throws with the same text the description carries`() {
        val subject = report(everyCheckBusy - "navigation")

        val error = kotlin.runCatching { TckGate.require(subject) }.exceptionOrNull()
        assertTrue(error is AssertionError, "require passed a run in which navigation had no target")
        assertTrue(error.message!!.contains("navigation"))
    }
}
