package tacku.app

import kotlin.io.encoding.Base64
import kotlin.io.encoding.ExperimentalEncodingApi
import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Whether a stored token is worth presenting.
 *
 * The question exists because of a loop nobody could see from inside the page: a token was kept
 * after it expired, handed back on every start, and refused every time — while the provider went on
 * signing the person in perfectly. From the outside it looked like "it logs me in and then says
 * 401", which is exactly what it was.
 */
class TokenLifeTest {
    @OptIn(ExperimentalEncodingApi::class)
    private fun tokenExpiring(at: Long): String {
        val payload = """{"sub":"anna","exp":$at}"""
        val encoded = Base64.UrlSafe.withPadding(Base64.PaddingOption.ABSENT).encode(payload.encodeToByteArray())
        return "header.$encoded.signature"
    }

    @Test
    fun `a token past its expiry is spent`() {
        assertTrue(TokenLife.isSpent(tokenExpiring(1_000), nowSeconds = 2_000))
    }

    @Test
    fun `a token with minutes left is not`() {
        assertFalse(TokenLife.isSpent(tokenExpiring(2_000), nowSeconds = 1_000))
    }

    @Test
    fun `a token about to expire counts as spent`() {
        // Presenting one with two seconds left buys a refusal in flight rather than a request: the
        // margin is what turns a race into a decision.
        assertTrue(TokenLife.isSpent(tokenExpiring(1_002), nowSeconds = 1_000))
    }

    @Test
    fun `a token this cannot read is presented rather than discarded`() {
        // Refusing what cannot be parsed would turn an unfamiliar shape into a sign-in loop. The
        // server explains a refusal; a loop explains nothing.
        assertFalse(TokenLife.isSpent("not-a-token", nowSeconds = 1_000))
        assertFalse(TokenLife.isSpent("header.%%%.signature", nowSeconds = 1_000))
    }
}
