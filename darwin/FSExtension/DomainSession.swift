import Apollo
import FileProvider
import PlatriumCore
import PlatriumSDK

/// Everything one File Provider domain needs to talk to its server: which
/// account it belongs to (the domain identifier is the account id), a GraphQL
/// client, and a way to build transfer clients. The token is read from the
/// shared Keychain on use, so signing in again in the app takes effect without
/// restarting the extension.
final class DomainSession: @unchecked Sendable {
    let account: Account
    let server: Server
    let apollo: ApolloClient

    private let vault: any TokenVault

    private init(account: Account, server: Server, apollo: ApolloClient, vault: any TokenVault) {
        self.account = account
        self.server = server
        self.apollo = apollo
        self.vault = vault
    }

    /// Looks the domain's account up in the store shared with the app. Throws
    /// `notAuthenticated` when the domain has no account, which makes the system
    /// show it as signed out instead of retrying forever.
    static func open(domainIdentifier: String) throws -> DomainSession {
        let repository = AccountRepository(database: try AppDatabase.openShared())
        guard let account = try repository.account(forDomain: domainIdentifier),
              let server = try repository.server(id: account.serverId)
        else { throw NSFileProviderError(.notAuthenticated) }

        let vault = KeychainTokenVault()
        let apollo = try ClientFactory.makeApollo(server: server, account: account, vault: vault) { accountId in
            // The app picks this up when it returns to the foreground.
            try? repository.setStatus(.needsReauth, forAccount: accountId)
        }
        return DomainSession(account: account, server: server, apollo: apollo, vault: vault)
    }

    /// A transfer helper carrying the account's current token.
    func contentTransfer() throws -> ContentTransferUtility {
        ContentTransferUtility(client: try ClientFactory.makeSDK(server: server, account: account, vault: vault))
    }

    /// Maps a failure to what the system should show: "signed out" when the
    /// token is missing or rejected, "server unreachable" otherwise.
    func fileProviderError(_ error: Error) -> Error {
        if isAuthenticationFailure(error) { return NSFileProviderError(.notAuthenticated) }
        return NSError(
            domain: NSCocoaErrorDomain,
            code: NSFileProviderError.serverUnreachable.rawValue,
            userInfo: [NSUnderlyingErrorKey: error]
        )
    }

    /// Like `fileProviderError`, but leaves other errors untouched.
    func mapAuthentication(_ error: Error) -> Error {
        isAuthenticationFailure(error) ? NSFileProviderError(.notAuthenticated) : error
    }

    private func isAuthenticationFailure(_ error: Error) -> Bool {
        if case ClientError.noToken = error { return true }
        if let http = error as? ResponseCodeInterceptor.ResponseCodeError { return http.response.statusCode == 401 }
        if let sdk = error as? PlatriumError, case .Unauthorized = sdk { return true }
        return false
    }
}
