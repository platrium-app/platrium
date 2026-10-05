package org.platrium.platrium.core

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.platrium.platrium.core.account.Account
import org.platrium.platrium.core.account.AccountDao
import org.platrium.platrium.core.account.AccountRepository
import org.platrium.platrium.core.account.AccountStatus
import org.platrium.platrium.core.account.Server
import org.platrium.platrium.core.account.Setting

/** An in-memory [AccountDao]. */
class FakeAccountDao : AccountDao {
    val servers = MutableStateFlow<List<Server>>(emptyList())
    val accounts = MutableStateFlow<List<Account>>(emptyList())
    val settings = MutableStateFlow<List<Setting>>(emptyList())

    override fun observeServers(): Flow<List<Server>> = servers
    override fun observeAccounts(): Flow<List<Account>> = accounts
    override fun observeSettings(): Flow<List<Setting>> = settings
    override suspend fun serverByUrl(url: String) = servers.value.firstOrNull { it.url == url }
    override suspend fun server(id: String) = servers.value.firstOrNull { it.id == id }
    override suspend fun upsert(server: Server) { servers.value = servers.value.filterNot { it.id == server.id } + server }
    override suspend fun account(id: String) = accounts.value.firstOrNull { it.id == id }
    override suspend fun findAccount(serverId: String, tenantId: String, userId: String) =
        accounts.value.firstOrNull { it.serverId == serverId && it.remoteTenantId == tenantId && it.remoteUserId == userId }
    override suspend fun upsert(account: Account) { accounts.value = accounts.value.filterNot { it.id == account.id } + account }
    override suspend fun setStatus(id: String, status: AccountStatus) {
        accounts.value = accounts.value.map { if (it.id == id) it.copy(status = status) else it }
    }
    override suspend fun setting(key: String) = settings.value.firstOrNull { it.key == key }?.value
    override suspend fun upsert(setting: Setting) { settings.value = settings.value.filterNot { it.key == setting.key } + setting }
    override suspend fun deleteSetting(key: String) { settings.value = settings.value.filterNot { it.key == key } }
}

class AccountRepositoryTest {
    private val dao = FakeAccountDao()
    private val repo = AccountRepository(dao)

    @Test
    fun addingTheSameServerTwiceReturnsTheExistingOne() = runTest {
        val a = repo.addServer("files.example.com")
        val b = repo.addServer("https://files.example.com/")
        assertEquals(a.id, b.id)
        assertEquals(1, dao.servers.value.size)
        assertEquals("files.example.com", a.name)
    }

    @Test
    fun signingInAgainKeepsTheAccountIdAndReactivatesIt() = runTest {
        val server = repo.addServer("files.example.com")
        val first = repo.saveAccount("id-1", server.id, "t", "u", "a@x.com", "tok-1", "dev-1")
        repo.setStatus(first.id, AccountStatus.NEEDS_REAUTH)

        val again = repo.saveAccount("id-2", server.id, "t", "u", "a@x.com", "tok-2", "dev-2")

        assertEquals("id-1", again.id)
        assertEquals(AccountStatus.ACTIVE, again.status)
        assertEquals("tok-2", again.tokenId)
        assertEquals(1, dao.accounts.value.size)
    }

    @Test
    fun twoUsersOnOneServerAreTwoAccounts() = runTest {
        val server = repo.addServer("files.example.com")
        repo.saveAccount("a", server.id, "t", "u1", "a@x.com", "1", null)
        repo.saveAccount("b", server.id, "t", "u2", "b@x.com", "2", null)
        assertEquals(2, dao.accounts.value.size)
    }

    @Test
    fun selectionFallsBackWhenTheStoredAccountIsGone() {
        val s1 = Server(id = "s1", name = "one", url = "https://one", createdAt = 1)
        val s2 = Server(id = "s2", name = "two", url = "https://two", createdAt = 2)
        val a2 = Account(id = "a2", serverId = "s2", remoteTenantId = "t", remoteUserId = "u", email = "e", tokenId = "k", deviceId = null)

        // Stored account missing, stored server present: that server's first account.
        val sel = AccountRepository.resolveSelection(
            listOf(s1, s2), listOf(a2),
            mapOf(AccountRepository.ACTIVE_ACCOUNT to "gone", AccountRepository.ACTIVE_SERVER to "s2"),
        )
        assertEquals("a2", sel.account?.id)
        assertEquals("s2", sel.server?.id)

        // Nothing stored: first account anywhere.
        assertEquals("a2", AccountRepository.resolveSelection(listOf(s1, s2), listOf(a2), emptyMap()).account?.id)

        // Only servers: the first server, no account.
        val onlyServers = AccountRepository.resolveSelection(listOf(s1, s2), emptyList(), emptyMap())
        assertEquals("s1", onlyServers.server?.id)
        assertNull(onlyServers.account)

        // Nothing at all.
        assertNull(AccountRepository.resolveSelection(emptyList(), emptyList(), emptyMap()).server)
    }

    @Test
    fun selectingAServerDropsTheActiveAccountOfAnotherServer() = runTest {
        val s1 = repo.addServer("one.example.com")
        val s2 = repo.addServer("two.example.com")
        val a = repo.saveAccount("a", s1.id, "t", "u", "e", "k", null)
        repo.setActive(a.id)
        repo.setActiveServer(s2.id)
        assertNull(dao.setting(AccountRepository.ACTIVE_ACCOUNT))
    }
}
