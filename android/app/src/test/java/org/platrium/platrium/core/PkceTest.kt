package org.platrium.platrium.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.platrium.platrium.core.auth.Pkce

class PkceTest {
    @Test
    fun challengeMatchesRfc7636Vector() {
        // RFC 7636, appendix B.
        assertEquals(
            "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
            Pkce.challengeFor("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"),
        )
    }

    @Test
    fun verifiersAreLongUrlSafeAndUnique() {
        val a = Pkce()
        val b = Pkce()
        assertTrue(a.verifier.length in 43..128)
        assertTrue(a.verifier.matches(Regex("[A-Za-z0-9_-]+")))
        assertNotEquals(a.verifier, b.verifier)
        assertEquals(Pkce.challengeFor(a.verifier), a.challenge)
    }
}
