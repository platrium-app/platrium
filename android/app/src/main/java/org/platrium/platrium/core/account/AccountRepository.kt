package org.platrium.platrium.core.account

import java.net.URI
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine

/** Reads and writes servers, accounts and the active selection. */
class AccountRepository(private val dao: AccountDao) {
    val servers: Flow<List<Server>> = dao.observeServers()
    val accounts: Flow<List<Account>> = dao.observeAccounts()

    val selection: Flow<Selection> =
        combine(dao.observeServers(), dao.observeAccounts(), dao.observeSettings()) { servers, accounts, settings ->
            resolveSelection(servers, accounts, settings.associate { it.key to it.value })
        }

    suspend fun server(id: String): Server? = dao.server(id)
    suspend fun account(id: String): Account? = dao.account(id)

    /** Adds a server, or returns the existing one with the same URL. */
    suspend fun addServer(url: String, name: String? = null): Server {
        val normalized = ServerUrl.normalize(url)
        dao.serverByUrl(normalized)?.let { return it }
        val host = runCatching { URI(normalized).host }.getOrNull() ?: normalized
        val server = Server(name = name?.trim()?.takeIf { it.isNotEmpty() } ?: host, url = normalized)
        dao.upsert(server)
        return server
    }

    /**
     * Inserts the account or, when this user is already signed in on this
     * server, replaces its credential details and marks it active again. The
     * id of an existing account never changes: its token is keyed by it.
     */
    suspend fun saveAccount(
        id: String,
        serverId: String,
        tenantId: String,
        userId: String,
        email: String,
        tokenId: String,
        deviceId: String?,
    ): Account {
        val existing = dao.findAccount(serverId, tenantId, userId)
        val account = existing?.copy(
            email = email,
            tokenId = tokenId,
            deviceId = deviceId,
            status = AccountStatus.ACTIVE,
            lastUsedAt = System.currentTimeMillis(),
        ) ?: Account(
            id = id,
            serverId = serverId,
            remoteTenantId = tenantId,
            remoteUserId = userId,
            email = email,
            tokenId = tokenId,
            deviceId = deviceId,
        )
        dao.upsert(account)
        return account
    }

    suspend fun findAccount(serverId: String, tenantId: String, userId: String): Account? =
        dao.findAccount(serverId, tenantId, userId)

    suspend fun setStatus(accountId: String, status: AccountStatus) = dao.setStatus(accountId, status)

    suspend fun setActive(accountId: String) {
        val account = dao.account(accountId) ?: return
        dao.upsert(Setting(ACTIVE_ACCOUNT, account.id))
        dao.upsert(Setting(ACTIVE_SERVER, account.serverId))
    }

    /** Selects a server that may have no accounts yet. */
    suspend fun setActiveServer(serverId: String) {
        if (dao.server(serverId) == null) return
        dao.upsert(Setting(ACTIVE_SERVER, serverId))
        val current = dao.setting(ACTIVE_ACCOUNT)
        if (current != null && dao.account(current)?.serverId != serverId) dao.deleteSetting(ACTIVE_ACCOUNT)
    }

    companion object {
        const val ACTIVE_ACCOUNT = "active_account_id"
        const val ACTIVE_SERVER = "active_server_id"

        /**
         * The selection to show, repaired when what was stored no longer
         * exists: the stored account, else the first account of the stored
         * server, else the first account anywhere, else the first server.
         */
        fun resolveSelection(servers: List<Server>, accounts: List<Account>, settings: Map<String, String>): Selection {
            val serversById = servers.associateBy { it.id }
            settings[ACTIVE_ACCOUNT]?.let { id ->
                val account = accounts.firstOrNull { it.id == id }
                val server = account?.let { serversById[it.serverId] }
                if (account != null && server != null) return Selection(server, account)
            }
            settings[ACTIVE_SERVER]?.let { id ->
                serversById[id]?.let { server ->
                    return Selection(server, accounts.firstOrNull { it.serverId == server.id })
                }
            }
            accounts.firstOrNull { serversById.containsKey(it.serverId) }?.let {
                return Selection(serversById.getValue(it.serverId), it)
            }
            return Selection(servers.firstOrNull(), null)
        }
    }
}
