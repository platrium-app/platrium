package org.platrium.platrium.core

import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.platrium.platrium.core.account.AccountRepository
import org.platrium.platrium.core.account.Server
import org.platrium.platrium.core.auth.AuthBackend
import org.platrium.platrium.core.auth.DeviceDescriptor
import org.platrium.platrium.core.auth.InMemoryTokenVault
import org.platrium.platrium.core.auth.Pkce
import org.platrium.platrium.core.auth.SignInException
import org.platrium.platrium.core.auth.SignInService
import uniffi.platrium_sdk.AuthKind
import uniffi.platrium_sdk.Identity
import uniffi.platrium_sdk.PlatriumException
import uniffi.platrium_sdk.TokenGrant

private class FakeBackend(
    var identity: Identity = Identity("user-1", "tenant-1", "me@example.com", AuthKind.DEVICE, "dev-1"),
    var failExchange: PlatriumException? = null,
) : AuthBackend {
    var token = "plt_first"
    var tokenId = "tok-1"
    var lastVerifier: String? = null

    override suspend fun exchangeCode(server: Server, code: String, verifier: String): TokenGrant {
        failExchange?.let { throw it }
        lastVerifier = verifier
        return TokenGrant(token, tokenId, identity.deviceId, null)
    }

    override suspend fun identity(server: Server, token: String) = identity
}

class SignInServiceTest {
    private val dao = FakeAccountDao()
    private val repo = AccountRepository(dao)
    private val vault = InMemoryTokenVault()
    private val backend = FakeBackend()
    private val service = SignInService(repo, vault, DeviceDescriptor("Platrium on Pixel", "1.0"), backend)

    /** Plays the browser: reads state and challenge from the URL and redirects back with a code. */
    private fun browser(code: String = "the-code", state: (String) -> String = { it }): suspend (String) -> String = { url ->
        val params = url.substringAfter('?').split('&').associate { it.substringBefore('=') to it.substringAfter('=') }
        "platrium://auth/callback?code=$code&state=${state(params.getValue("state"))}"
    }

    private suspend fun server() = repo.addServer("files.example.com")

    @Test
    fun happyPathStoresTheTokenInTheVaultAndTheAccountInTheDatabase() = runTest {
        val server = server()
        val result = service.signIn(server, browser())

        assertTrue(result.isNewAccount)
        assertEquals("me@example.com", result.account.email)
        assertEquals("plt_first", vault.token(result.account.id))
        assertEquals(1, dao.accounts.value.size)
        // The verifier sent to the server is the one the challenge in the URL was made from.
        assertEquals(43, backend.lastVerifier?.length)
    }

    @Test
    fun theAuthorizeUrlCarriesPkceAndTheDevice() = runTest {
        val server = server()
        var seen = ""
        service.signIn(server) { url -> seen = url; browser()(url) }

        assertTrue(seen.startsWith("https://files.example.com/authorize?"))
        assertTrue(seen.contains("redirect_uri=platrium%3A%2F%2Fauth%2Fcallback"))
        assertTrue(seen.contains("platform=ANDROID"))
        assertTrue(seen.contains("name=Platrium+on+Pixel"))
        val challenge = seen.substringAfter("code_challenge=").substringBefore('&')
        assertEquals(Pkce.challengeFor(backend.lastVerifier!!), challenge)
    }

    @Test
    fun aStateMismatchIsRejectedBeforeAnythingIsExchanged() = runTest {
        val server = server()
        assertThrows(SignInException.StateMismatch::class.java) {
            kotlinx.coroutines.runBlocking { service.signIn(server, browser(state = { "forged" })) }
        }
        assertNull(backend.lastVerifier)
        assertTrue(dao.accounts.value.isEmpty())
    }

    @Test
    fun anErrorRedirectIsReportedWithItsReason() = runTest {
        val server = server()
        val e = assertThrows(SignInException.Denied::class.java) {
            kotlinx.coroutines.runBlocking {
                service.signIn(server) { "platrium://auth/callback?error=access_denied&error_description=User+said+no" }
            }
        }
        assertTrue(e.message!!.contains("User said no"))
    }

    @Test
    fun aFailedExchangeLeavesNothingBehind() = runTest {
        val server = server()
        backend.failExchange = PlatriumException.BadRequest("invalid_grant")
        assertThrows(SignInException.Server::class.java) {
            kotlinx.coroutines.runBlocking { service.signIn(server, browser()) }
        }
        assertTrue(dao.accounts.value.isEmpty())
    }

    @Test
    fun signingInAgainReplacesTheTokenAndKeepsOneAccount() = runTest {
        val server = server()
        val first = service.signIn(server, browser())
        backend.token = "plt_second"
        backend.tokenId = "tok-2"
        val second = service.signIn(server, browser())

        assertFalse(second.isNewAccount)
        assertEquals(first.account.id, second.account.id)
        assertEquals("plt_second", vault.token(first.account.id))
        assertEquals("tok-2", second.account.tokenId)
        assertEquals(1, dao.accounts.value.size)
    }

    @Test
    fun codeParsingDecodesAndValidates() {
        assertEquals("a b", SignInService.code("platrium://auth/callback?state=s&code=a+b", "s"))
        assertThrows(SignInException.MissingCode::class.java) { SignInService.code("platrium://auth/callback?state=s", "s") }
    }
}
