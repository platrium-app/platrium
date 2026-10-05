package org.platrium.platrium.core.auth

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

/** A PKCE (RFC 7636) verifier and its S256 challenge. */
class Pkce(val verifier: String = randomToken()) {
    val challenge: String = challengeFor(verifier)

    companion object {
        private val random = SecureRandom()
        private val encoder = Base64.getUrlEncoder().withoutPadding()

        fun challengeFor(verifier: String): String =
            encoder.encodeToString(MessageDigest.getInstance("SHA-256").digest(verifier.toByteArray(Charsets.US_ASCII)))

        fun randomState(): String = randomToken()

        private fun randomToken(): String = encoder.encodeToString(ByteArray(32).also(random::nextBytes))
    }
}
