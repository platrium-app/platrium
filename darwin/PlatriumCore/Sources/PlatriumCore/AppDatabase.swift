import Foundation
import GRDB

public enum AppDatabaseError: LocalizedError {
    case noAppGroupContainer(String)

    public var errorDescription: String? {
        switch self {
        case .noAppGroupContainer(let id): "The app group container “\(id)” is not available."
        }
    }
}

/// The SQLite file shared by the app and the File Provider extension.
public final class AppDatabase: Sendable {
    public static let defaultAppGroup = "group.org.platrium"

    public let writer: any DatabaseWriter

    public init(_ writer: any DatabaseWriter) throws {
        self.writer = writer
        try Self.migrator.migrate(writer)
    }

    /// Opens (creating if needed) the database in the app group container. Both
    /// processes use WAL, so one can read while the other writes.
    public static func openShared(appGroup: String = defaultAppGroup) throws -> AppDatabase {
        guard let container = FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: appGroup) else {
            throw AppDatabaseError.noAppGroupContainer(appGroup)
        }
        let directory = container.appending(path: "Library/Application Support/Platrium", directoryHint: .isDirectory)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)

        var config = Configuration()
        // Two processes share the file; wait for the other's write instead of failing.
        config.busyMode = .timeout(5)
        let pool = try DatabasePool(path: directory.appending(path: "platrium.sqlite").path, configuration: config)
        return try AppDatabase(pool)
    }

    /// A private in-memory database, for tests and previews.
    public static func inMemory() throws -> AppDatabase {
        try AppDatabase(DatabaseQueue())
    }

    static var migrator: DatabaseMigrator {
        var migrator = DatabaseMigrator()

        migrator.registerMigration("v1") { db in
            try db.create(table: "server") { t in
                t.primaryKey("id", .text)
                t.column("name", .text).notNull()
                t.column("url", .text).notNull().unique()
                t.column("created_at", .datetime).notNull()
            }

            try db.create(table: "account") { t in
                // Also the Keychain item key and the File Provider domain id.
                t.primaryKey("id", .text)
                // Removing a server removes its accounts.
                t.column("server_id", .text).notNull().references("server", onDelete: .cascade)
                t.column("remote_tenant_id", .text).notNull()
                t.column("remote_user_id", .text).notNull()
                t.column("email", .text).notNull()
                t.column("token_id", .text).notNull()
                t.column("device_id", .text)
                t.column("status", .text).notNull().defaults(to: AccountStatus.active.rawValue)
                t.column("created_at", .datetime).notNull()
                t.column("last_used_at", .datetime).notNull()
                t.uniqueKey(["server_id", "remote_tenant_id", "remote_user_id"])
            }

            try db.create(table: "setting") { t in
                t.primaryKey("key", .text)
                t.column("value", .text).notNull()
            }
        }

        return migrator
    }
}
