import FileProvider
import OSLog
import PlatriumCore

/// Keeps one File Provider domain per account. The domain identifier is the
/// account id, so the extension can find its server and token from nothing
/// but the domain it was started for, and item ids from different servers or
/// users never share a domain.
enum FileProviderDomainManager {
    private static let logger = Logger(subsystem: "org.platrium.Platrium", category: "FileProviderDomains")

    /// Ids of the throwaway domains earlier development builds created.
    private static let legacyPrefix = "platrium_dev_"

    static func displayName(for account: Account, server: Server, multipleServers: Bool) -> String {
        multipleServers ? "Platrium – \(account.email) (\(server.host))" : "Platrium – \(account.email)"
    }

    /// Adds the account's domain if it does not exist yet, and renames it if the
    /// display name changed.
    static func ensureDomain(for account: Account, server: Server, multipleServers: Bool) async {
        let name = displayName(for: account, server: server, multipleServers: multipleServers)
        let id = NSFileProviderDomainIdentifier(rawValue: account.id)
        do {
            let existing = try await NSFileProviderManager.domains()
            if existing.contains(where: { $0.identifier == id }) { return }
            try await NSFileProviderManager.add(NSFileProviderDomain(identifier: id, displayName: name))
            logger.info("Added File Provider domain for account \(account.id, privacy: .public)")
        } catch {
            logger.error("Could not add File Provider domain: \(error.localizedDescription, privacy: .public)")
        }
    }

    /// Makes sure every account has a domain, and removes the leftover
    /// development domains. Domains that belong to no account but are not ours
    /// to judge are left alone.
    static func reconcile(accounts: [Account], servers: [Server]) async {
        do {
            let existing = try await NSFileProviderManager.domains()
            for domain in existing where domain.identifier.rawValue.hasPrefix(legacyPrefix) {
                try await NSFileProviderManager.remove(domain)
            }
        } catch {
            logger.error("Could not clean up old domains: \(error.localizedDescription, privacy: .public)")
        }
        let serversById = Dictionary(uniqueKeysWithValues: servers.map { ($0.id, $0) })
        for account in accounts {
            guard let server = serversById[account.serverId] else { continue }
            await ensureDomain(for: account, server: server, multipleServers: servers.count > 1)
        }
    }

    /// Tells the system an account that could not authenticate can now try again.
    static func signalSignedBackIn(_ account: Account) async {
        let id = NSFileProviderDomainIdentifier(rawValue: account.id)
        guard let domain = try? await NSFileProviderManager.domains().first(where: { $0.identifier == id }),
              let manager = NSFileProviderManager(for: domain) else { return }
        try? await manager.signalErrorResolved(NSFileProviderError(.notAuthenticated))
        try? await manager.signalEnumerator(for: .workingSet)
    }
}
