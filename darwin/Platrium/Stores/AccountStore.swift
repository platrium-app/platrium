import Apollo
import Foundation
import Observation
import OSLog
import PlatriumCore
import PlatriumSDK

/// The app's view of servers and signed-in accounts. State lives in the SQLite
/// file shared with the File Provider extension; tokens live in the Keychain.
@Observable
final class AccountStore {
    private(set) var servers: [Server] = []
    private(set) var accounts: [Account] = []
    private(set) var activeServer: Server?
    private(set) var activeAccount: Account?

    /// Clients for the active account. Nil until someone is signed in.
    private(set) var apollo: ApolloClient?
    private(set) var sdk: PlatriumClient?

    /// Set when the shared store could not be opened. The app cannot work without it.
    private(set) var storageError: String?

    var isShowingAccountSwitcher = false

    private let repository: AccountRepository
    private let vault: any TokenVault
    private let signInService: SignInService
    private var clientsKey: String?
    private let logger = Logger(subsystem: "org.platrium.Platrium", category: "AccountStore")

    init() {
        let vault = KeychainTokenVault()
        let database: AppDatabase
        do {
            database = try AppDatabase.openShared()
        } catch {
            storageError = error.localizedDescription
            // Keep the app launchable; it shows the error instead of the content.
            database = try! AppDatabase.inMemory()
        }
        let repository = AccountRepository(database: database)
        self.vault = vault
        self.repository = repository
        self.signInService = SignInService(repository: repository, vault: vault)
        reload()
    }

    // MARK: Lifecycle

    /// Called once at launch.
    func start() async {
        await FileProviderDomainManager.reconcile(accounts: accounts, servers: servers)
    }

    /// Re-reads the shared store. Also called when the app returns to the
    /// foreground, because the File Provider extension may have marked an
    /// account as signed out.
    func reload() {
        do {
            servers = try repository.servers()
            accounts = try repository.accounts()
            let selection = try repository.selection()
            activeServer = selection.server
            activeAccount = selection.account
        } catch {
            logger.error("Could not read accounts: \(error.localizedDescription, privacy: .public)")
        }
        rebuildClients()
    }

    func accounts(for server: Server) -> [Account] {
        accounts.filter { $0.serverId == server.id }
    }

    // MARK: Servers and accounts

    /// Checks that the address is a Platrium server, then remembers it and makes it the active server.
    func addServer(address: String) async throws -> (server: Server, details: ServerDetails) {
        let normalized = try ServerURL.normalize(address)
        let details = try await ServerProbe.fetch(serverURL: normalized)
        let server = try repository.addServer(url: normalized)
        try repository.setActive(serverId: server.id)
        reload()
        return (server, details)
    }

    func selectAccount(_ account: Account) {
        try? repository.setActive(accountId: account.id)
        reload()
    }

    /// Signs a user in on `server` through the browser, registering this
    /// installation as a device, and makes that account active.
    func signIn(to server: Server, authenticate: @escaping BrowserAuthenticator) async throws {
        let result = try await signInService.signIn(to: server, authenticate: authenticate)
        try repository.setActive(accountId: result.account.id)
        reload()
        clientsKey = nil
        rebuildClients() // the token changed, and the SDK client has it baked in

        await FileProviderDomainManager.ensureDomain(for: result.account, server: server, multipleServers: servers.count > 1)
        if !result.isNewAccount {
            await FileProviderDomainManager.signalSignedBackIn(result.account)
        }
    }

    /// The server (or the vault) said this account's token no longer works.
    func markNeedsReauth(_ accountId: String) {
        guard accounts.first(where: { $0.id == accountId })?.status == .active else { return }
        try? repository.setStatus(.needsReauth, forAccount: accountId)
        reload()
    }

    // MARK: Clients

    private func rebuildClients() {
        guard let server = activeServer, let account = activeAccount, account.status == .active else {
            apollo = nil
            sdk = nil
            clientsKey = nil
            return
        }
        let key = account.id + "|" + account.tokenId
        guard key != clientsKey else { return }

        do {
            apollo = try ClientFactory.makeApollo(server: server, account: account, vault: vault) { [weak self] id in
                Task { @MainActor in self?.markNeedsReauth(id) }
            }
            sdk = try ClientFactory.makeSDK(server: server, account: account, vault: vault)
            clientsKey = key
        } catch ClientError.noToken {
            markNeedsReauth(account.id)
        } catch {
            logger.error("Could not create clients: \(error.localizedDescription, privacy: .public)")
            apollo = nil
            sdk = nil
        }
    }
}
