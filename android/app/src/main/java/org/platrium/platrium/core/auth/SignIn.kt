package org.platrium.platrium.core.auth

import java.net.URLDecoder
import java.net.URLEncoder
import java.util.UUID
import org.platrium.platrium.core.account.Account
import org.platrium.platrium.core.account.AccountRepository
import org.platrium.platrium.core.account.Server
import uniffi.platrium_sdk.Identity
import uniffi.platrium_sdk.PlatriumClient
import uniffi.platrium_sdk.PlatriumException
import uniffi.platrium_sdk.TokenGrant

/** The auth calls the Rust SDK exposes; an API change breaks the build here. */
interface AuthBackend {
    suspend fun exchangeCode(server: Server, code: String, verifier: String): TokenGrant
    suspend fun identity(server: Server, token: String): Identity
}

class SdkAuthBackend : AuthBackend {
    override suspend fun exchangeCode(server: Server, code: String, verifier: String): TokenGrant =
        PlatriumClient(server.restBaseUrl).use { it.auth().exchangeCode(code, verifier) }

    override suspend fun identity(server: Server, token: String): Identity =
        PlatriumClient.withToken(server.restBaseUrl, token).use { it.auth().me() }
}

sealed class SignInException(message: String) : Exception(message) {
    object Cancelled : SignInException("Sign-in was cancelled.")
    object StateMismatch : SignInException("The sign-in response didn't match the request. Please try again.")
    class Denied(reason: String) : SignInException("Sign-in was refused: $reason.")
    object MissingCode : SignInException("The server didn't return a sign-in code.")
    class Server(message: String) : SignInException(message)
}

/** Who this install tells the server it is; shown on the web under Devices & Apps. */
data class DeviceDescriptor(val name: String, val appVersion: String, val platform: String = "ANDROID")

data class SignInResult(val account: Account, val isNewAccount: Boolean)

/** Opens [authorizeUrl] in a browser and returns the URL the server redirected back to. */
typealias BrowserAuthenticator = suspend (authorizeUrl: String) -> String

/**
 * The browser grant: the user signs in on the server's own web page and
 * presses Allow; the app gets a single-use code (PKCE-bound) it exchanges for
 * a token, which goes into the [TokenVault].
 */
class SignInService(
    private val repository: AccountRepository,
    private val vault: TokenVault,
    private val device: DeviceDescriptor,
    private val backend: AuthBackend = SdkAuthBackend(),
) {
    suspend fun signIn(server: Server, authenticate: BrowserAuthenticator): SignInResult {
        val pkce = Pkce()
        val state = Pkce.randomState()

        val callback = authenticate(authorizeUrl(server, pkce.challenge, state))
        val code = code(callback, state)

        val grant: TokenGrant
        val identity: Identity
        try {
            grant = backend.exchangeCode(server, code, pkce.verifier)
            identity = backend.identity(server, grant.token)
        } catch (e: PlatriumException) {
            throw SignInException.Server(describe(e))
        }

        // Re-signing in keeps the account's id: its token is keyed by it.
        val existing = repository.findAccount(server.id, identity.tenantId, identity.userId)
        val accountId = existing?.id ?: UUID.randomUUID().toString()

        // Token first: a row without a token is useless, a token without a
        // row is harmless and gets overwritten next time.
        vault.setToken(accountId, grant.token)
        try {
            val account = repository.saveAccount(
                id = accountId,
                serverId = server.id,
                tenantId = identity.tenantId,
                userId = identity.userId,
                email = identity.email,
                tokenId = grant.tokenId,
                deviceId = grant.deviceId,
            )
            return SignInResult(account, isNewAccount = existing == null)
        } catch (e: Exception) {
            if (existing == null) vault.deleteToken(accountId)
            throw e
        }
    }

    internal fun authorizeUrl(server: Server, challenge: String, state: String): String {
        val query = listOf(
            "redirect_uri" to REDIRECT_URI,
            "code_challenge" to challenge,
            "state" to state,
            "name" to device.name,
            "platform" to device.platform,
            "app_version" to device.appVersion,
        ).joinToString("&") { (k, v) -> "$k=${URLEncoder.encode(v, "UTF-8")}" }
        return "${server.url}/authorize?$query"
    }

    companion object {
        const val CALLBACK_SCHEME = "platrium"
        const val REDIRECT_URI = "$CALLBACK_SCHEME://auth/callback"

        internal fun describe(error: PlatriumException): String = when (error) {
            is PlatriumException.Unauthorized -> "The server didn't accept the sign-in. Please try again."
            is PlatriumException.BadRequest -> "The sign-in expired or was already used. Please try again."
            is PlatriumException.ApiException ->
                if (error.v1.contains("HTTP 5")) {
                    "The server ran into a problem while signing you in. Please try again, or contact your administrator."
                } else {
                    "Couldn't complete sign-in: ${error.v1}"
                }
            is PlatriumException.InternalException -> "Couldn't complete sign-in: ${error.v1}"
        }

        internal fun code(callback: String, expectedState: String): String {
            val query = callback.substringAfter('?', "").substringBefore('#')
            val params = query.split('&').filter { it.isNotEmpty() }.associate {
                val key = it.substringBefore('=')
                val value = it.substringAfter('=', "")
                URLDecoder.decode(key, "UTF-8") to URLDecoder.decode(value, "UTF-8")
            }
            params["error"]?.let { throw SignInException.Denied(params["error_description"] ?: it) }
            if (params["state"] != expectedState) throw SignInException.StateMismatch
            return params["code"]?.takeIf { it.isNotEmpty() } ?: throw SignInException.MissingCode
        }
    }
}
