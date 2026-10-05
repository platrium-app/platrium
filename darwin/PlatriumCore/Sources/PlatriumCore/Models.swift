import Foundation
import GRDB

/// A Platrium server the app can talk to.
public struct Server: Codable, Hashable, Identifiable, Sendable, FetchableRecord, PersistableRecord {
    public static let databaseTableName = "server"
    public static let databaseColumnDecodingStrategy = DatabaseColumnDecodingStrategy.convertFromSnakeCase
    public static let databaseColumnEncodingStrategy = DatabaseColumnEncodingStrategy.convertToSnakeCase

    public var id: String
    public var name: String
    /// Normalized: scheme, host, optional port and path, no trailing slash.
    public var url: String
    public var createdAt: Date

    public init(id: String = UUID().uuidString, name: String, url: String, createdAt: Date = Date()) {
        self.id = id
        self.name = name
        self.url = url
        self.createdAt = createdAt
    }

    public var restBaseURL: String { url + "/api" }
    public var graphQLURL: URL? { URL(string: url + "/graphql") }
    public var host: String { URL(string: url)?.host() ?? url }
}

public enum AccountStatus: String, Codable, Sendable {
    /// The token works as far as we know.
    case active
    /// The server rejected the token (revoked or expired); the user must sign in again.
    case needsReauth
}

/// One signed-in user on one server. The id doubles as the Keychain item key
/// and the File Provider domain identifier.
public struct Account: Codable, Hashable, Identifiable, Sendable, FetchableRecord, PersistableRecord {
    public static let databaseTableName = "account"
    public static let databaseColumnDecodingStrategy = DatabaseColumnDecodingStrategy.convertFromSnakeCase
    public static let databaseColumnEncodingStrategy = DatabaseColumnEncodingStrategy.convertToSnakeCase

    public var id: String
    public var serverId: String
    public var remoteTenantId: String
    public var remoteUserId: String
    public var email: String
    /// The server's id for this credential (`auth_tokens.id`).
    public var tokenId: String
    /// Set when the sign-in registered a device.
    public var deviceId: String?
    public var status: AccountStatus
    public var createdAt: Date
    public var lastUsedAt: Date

    public init(
        id: String = UUID().uuidString,
        serverId: String,
        remoteTenantId: String,
        remoteUserId: String,
        email: String,
        tokenId: String,
        deviceId: String?,
        status: AccountStatus = .active,
        createdAt: Date = Date(),
        lastUsedAt: Date = Date()
    ) {
        self.id = id
        self.serverId = serverId
        self.remoteTenantId = remoteTenantId
        self.remoteUserId = remoteUserId
        self.email = email
        self.tokenId = tokenId
        self.deviceId = deviceId
        self.status = status
        self.createdAt = createdAt
        self.lastUsedAt = lastUsedAt
    }
}
