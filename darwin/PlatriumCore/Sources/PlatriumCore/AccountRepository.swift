import Foundation
import GRDB

/// What the UI should treat as selected.
public struct Selection: Equatable, Sendable {
    public var server: Server?
    public var account: Account?
}

/// Reads and writes servers, accounts and the active selection.
public struct AccountRepository: Sendable {
    private enum SettingKey {
        static let activeAccount = "active_account_id"
        static let activeServer = "active_server_id"
    }

    private let database: AppDatabase
    private var writer: any DatabaseWriter { database.writer }

    public init(database: AppDatabase) {
        self.database = database
    }

    // MARK: Servers

    public func servers() throws -> [Server] {
        try writer.read { try Server.order(Column("created_at"), Column("id")).fetchAll($0) }
    }

    public func server(id: String) throws -> Server? {
        try writer.read { try Server.fetchOne($0, key: id) }
    }

    /// Adds a server, or returns the existing one with the same URL.
    @discardableResult
    public func addServer(name: String? = nil, url: String) throws -> Server {
        let normalized = try ServerURL.normalize(url)
        return try writer.write { db in
            if let existing = try Server.filter(Column("url") == normalized).fetchOne(db) {
                return existing
            }
            let host = URL(string: normalized)?.host() ?? normalized
            let trimmed = name?.trimmingCharacters(in: .whitespacesAndNewlines)
            let server = Server(name: (trimmed?.isEmpty == false ? trimmed : nil) ?? host, url: normalized)
            try server.insert(db)
            return server
        }
    }

    // MARK: Accounts

    public func accounts() throws -> [Account] {
        try writer.read { try Account.order(Column("created_at"), Column("id")).fetchAll($0) }
    }

    public func accounts(serverId: String) throws -> [Account] {
        try writer.read {
            try Account.filter(Column("server_id") == serverId).order(Column("created_at"), Column("id")).fetchAll($0)
        }
    }

    public func account(id: String) throws -> Account? {
        try writer.read { try Account.fetchOne($0, key: id) }
    }

    /// The File Provider domain identifier is the account id.
    public func account(forDomain domainIdentifier: String) throws -> Account? {
        try account(id: domainIdentifier)
    }

    public func account(serverId: String, tenantId: String, userId: String) throws -> Account? {
        try writer.read { db in
            try Account.filter(
                Column("server_id") == serverId && Column("remote_tenant_id") == tenantId && Column("remote_user_id") == userId
            ).fetchOne(db)
        }
    }

    /// Inserts the account, or, when this user is already signed in on this
    /// server, replaces its credential details and marks it active again. The
    /// id of an existing account never changes: the Keychain item and the File
    /// Provider domain are keyed by it.
    public func saveAccount(
        id: String,
        serverId: String,
        tenantId: String,
        userId: String,
        email: String,
        tokenId: String,
        deviceId: String?
    ) throws -> Account {
        try writer.write { db in
            if var existing = try Account.filter(
                Column("server_id") == serverId && Column("remote_tenant_id") == tenantId && Column("remote_user_id") == userId
            ).fetchOne(db) {
                existing.email = email
                existing.tokenId = tokenId
                existing.deviceId = deviceId
                existing.status = .active
                existing.lastUsedAt = Date()
                try existing.update(db)
                return existing
            }
            let account = Account(
                id: id, serverId: serverId, remoteTenantId: tenantId, remoteUserId: userId,
                email: email, tokenId: tokenId, deviceId: deviceId
            )
            try account.insert(db)
            return account
        }
    }

    public func setStatus(_ status: AccountStatus, forAccount id: String) throws {
        try writer.write { db in
            try db.execute(sql: "UPDATE account SET status = ? WHERE id = ?", arguments: [status.rawValue, id])
        }
    }

    // MARK: Selection

    public func activeAccountId() throws -> String? { try setting(SettingKey.activeAccount) }
    public func activeServerId() throws -> String? { try setting(SettingKey.activeServer) }

    public func setActive(accountId: String) throws {
        try writer.write { db in
            guard let account = try Account.fetchOne(db, key: accountId) else { return }
            try Self.put(SettingKey.activeAccount, account.id, in: db)
            try Self.put(SettingKey.activeServer, account.serverId, in: db)
        }
    }

    /// Selects a server that may have no accounts yet.
    public func setActive(serverId: String) throws {
        try writer.write { db in
            guard try Server.fetchOne(db, key: serverId) != nil else { return }
            try Self.put(SettingKey.activeServer, serverId, in: db)
            // Keep the active account only if it belongs to this server.
            if let current = try Self.get(SettingKey.activeAccount, in: db),
               try Account.fetchOne(db, key: current)?.serverId != serverId {
                try db.execute(sql: "DELETE FROM setting WHERE key = ?", arguments: [SettingKey.activeAccount])
            }
        }
    }

    /// The selection to show, repaired when what was stored no longer exists:
    /// the stored account, else the first account of the stored server, else
    /// the first account anywhere, else the first server, else nothing.
    public func selection() throws -> Selection {
        try writer.read { db in
            if let id = try Self.get(SettingKey.activeAccount, in: db),
               let account = try Account.fetchOne(db, key: id),
               let server = try Server.fetchOne(db, key: account.serverId) {
                return Selection(server: server, account: account)
            }
            if let id = try Self.get(SettingKey.activeServer, in: db),
               let server = try Server.fetchOne(db, key: id) {
                let first = try Account.filter(Column("server_id") == server.id)
                    .order(Column("created_at"), Column("id")).fetchOne(db)
                return Selection(server: server, account: first)
            }
            if let account = try Account.order(Column("created_at"), Column("id")).fetchOne(db),
               let server = try Server.fetchOne(db, key: account.serverId) {
                return Selection(server: server, account: account)
            }
            return Selection(server: try Server.order(Column("created_at"), Column("id")).fetchOne(db), account: nil)
        }
    }

    // MARK: Settings

    private func setting(_ key: String) throws -> String? {
        try writer.read { try Self.get(key, in: $0) }
    }

    private static func get(_ key: String, in db: Database) throws -> String? {
        try String.fetchOne(db, sql: "SELECT value FROM setting WHERE key = ?", arguments: [key])
    }

    private static func put(_ key: String, _ value: String, in db: Database) throws {
        try db.execute(
            sql: "INSERT INTO setting (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
            arguments: [key, value]
        )
    }
}
