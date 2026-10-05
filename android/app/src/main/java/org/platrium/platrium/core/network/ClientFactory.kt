package org.platrium.platrium.core.network

import com.apollographql.apollo.ApolloClient
import com.apollographql.apollo.api.http.HttpRequest
import com.apollographql.apollo.api.http.HttpResponse
import com.apollographql.apollo.network.http.HttpInterceptor
import com.apollographql.apollo.network.http.HttpInterceptorChain
import java.util.concurrent.ConcurrentHashMap
import org.platrium.platrium.core.account.Account
import org.platrium.platrium.core.account.Server
import org.platrium.platrium.core.auth.TokenVault
import org.platrium.platrium.graphql.GetServerInfoQuery
import uniffi.platrium_sdk.PlatriumClient

/** An account with the authenticated clients to talk to its server. */
class AccountSession(
    val account: Account,
    val server: Server,
    val apollo: ApolloClient,
    val sdk: PlatriumClient,
)

class NoTokenException : Exception("This account is signed out.")

/**
 * Builds the clients for an account and keeps them for as long as its
 * credentials don't change. The bearer token is read from the vault on every
 * request, and a 401 reports the account through [onUnauthorized].
 */
class ClientFactory(
    private val vault: TokenVault,
    private val onUnauthorized: (accountId: String) -> Unit,
    private val onSdkCreated: (PlatriumClient) -> Unit = {},
) {
    private data class Key(val accountId: String, val tokenId: String, val url: String)

    private val sessions = ConcurrentHashMap<Key, AccountSession>()

    fun session(account: Account, server: Server): AccountSession {
        val token = vault.token(account.id) ?: throw NoTokenException()
        val key = Key(account.id, account.tokenId, server.url)
        sessions.keys.removeAll { it.accountId == account.id && it != key }
        return sessions.getOrPut(key) {
            AccountSession(
                account = account,
                server = server,
                apollo = ApolloClient.Builder()
                    .serverUrl(server.graphqlUrl)
                    .addHttpInterceptor(BearerInterceptor(account.id, vault, onUnauthorized))
                    .build(),
                sdk = PlatriumClient.withToken(server.restBaseUrl, token).also(onSdkCreated),
            )
        }.let { if (it.account == account) it else AccountSession(account, server, it.apollo, it.sdk) }
    }
}

private class BearerInterceptor(
    private val accountId: String,
    private val vault: TokenVault,
    private val onUnauthorized: (String) -> Unit,
) : HttpInterceptor {
    override suspend fun intercept(request: HttpRequest, chain: HttpInterceptorChain): HttpResponse {
        val token = vault.token(accountId) ?: run {
            onUnauthorized(accountId)
            throw NoTokenException()
        }
        val response = chain.proceed(request.newBuilder().addHeader("Authorization", "Bearer $token").build())
        if (response.statusCode == 401) onUnauthorized(accountId)
        return response
    }
}

class ServerDetails(val version: String, val edition: String, val isMultiTenant: Boolean)

class ServerProbeException(message: String) : Exception(message)

/** Asks a not-yet-signed-in server who it is (public GraphQL `serverInfo`). */
object ServerProbe {
    suspend fun fetch(serverUrl: String): ServerDetails {
        val client = ApolloClient.Builder().serverUrl("$serverUrl/graphql").build()
        try {
            val response = client.query(GetServerInfoQuery()).execute()
            val info = response.data?.serverInfo
            if (info != null) return ServerDetails(info.version, info.edition, info.isMultiTenant)
            // A transport failure has no response data; an HTTP/parse failure means it isn't ours.
            val exception = response.exception
            throw ServerProbeException(
                if (exception is com.apollographql.apollo.exception.ApolloNetworkException) {
                    "Couldn't reach the server: ${exception.message ?: "network error"}"
                } else {
                    "That address doesn't look like a Platrium server."
                },
            )
        } finally {
            client.close()
        }
    }
}
