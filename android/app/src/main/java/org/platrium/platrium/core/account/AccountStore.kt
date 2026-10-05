package org.platrium.platrium.core.account

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import org.platrium.platrium.core.auth.CustomTabAuthenticator
import org.platrium.platrium.core.auth.SignInResult
import org.platrium.platrium.core.auth.SignInService
import org.platrium.platrium.core.network.AccountSession
import org.platrium.platrium.core.network.ClientFactory
import org.platrium.platrium.core.network.NoTokenException
import org.platrium.platrium.core.network.ServerDetails
import org.platrium.platrium.core.network.ServerProbe

/**
 * What the UI observes: the servers and accounts, which one is selected, and
 * the authenticated clients for the selected account. The single owner of
 * "who am I signed in as".
 */
class AccountStore(
    private val repository: AccountRepository,
    private val clients: ClientFactory,
    private val signIn: SignInService,
    private val authenticator: CustomTabAuthenticator,
    scope: CoroutineScope,
) {
    /** Null until the database has been read the first time. */
    val servers: StateFlow<List<Server>?> =
        repository.servers.stateIn(scope, SharingStarted.Eagerly, null)

    val accounts: StateFlow<List<Account>> =
        repository.accounts.stateIn(scope, SharingStarted.Eagerly, emptyList())

    val selection: StateFlow<Selection?> =
        repository.selection.stateIn(scope, SharingStarted.Eagerly, null)

    /** Clients for the selected account; null when there is none or it needs signing in. */
    val session: StateFlow<AccountSession?> =
        repository.selection
            .map { selection ->
                val account = selection.account
                val server = selection.server
                if (account == null || server == null || account.status != AccountStatus.ACTIVE) {
                    null
                } else {
                    try {
                        clients.session(account, server)
                    } catch (_: NoTokenException) {
                        repository.setStatus(account.id, AccountStatus.NEEDS_REAUTH)
                        null
                    }
                }
            }
            .distinctUntilChanged()
            .flowOn(Dispatchers.Default)
            .stateIn(scope, SharingStarted.Eagerly, null)

    /** Checks that [address] is a Platrium server and remembers it. */
    suspend fun addServer(address: String): Pair<Server, ServerDetails> {
        val url = ServerUrl.normalize(address)
        val details = ServerProbe.fetch(url)
        val server = repository.addServer(url)
        repository.setActiveServer(server.id)
        return server to details
    }

    suspend fun signIn(server: Server): SignInResult {
        val result = signIn.signIn(server, authenticator::authenticate)
        repository.setActive(result.account.id)
        return result
    }

    suspend fun select(accountId: String) = repository.setActive(accountId)

    fun accountsOf(serverId: String): List<Account> = accounts.value.filter { it.serverId == serverId }
}
