import Foundation
import Security

/// Where bearer tokens live. Only the token is secret, so it is kept out of the
/// SQLite file and in the Keychain, keyed by account id.
public protocol TokenVault: Sendable {
    func token(for accountId: String) throws -> String?
    func setToken(_ token: String, for accountId: String) throws
    func deleteToken(for accountId: String) throws
}

public struct KeychainError: LocalizedError {
    public let status: OSStatus
    public var errorDescription: String? {
        (SecCopyErrorMessageString(status, nil) as String?) ?? "Keychain error \(status)"
    }
}

/// Keychain-backed vault.
///
/// Items go in the app's shared keychain access group so the File Provider
/// extension can read what the app wrote. Both targets list the same group
/// first in their `keychain-access-groups` entitlement, and the system files
/// an item with no explicit group under the first one, so no team-prefixed
/// literal is needed in code. The item is available after the first unlock,
/// because the extension may run while the device is locked, and never leaves
/// the device or syncs through iCloud.
public struct KeychainTokenVault: TokenVault {
    public static let service = "org.platrium.token"

    public init() {}

    private func baseQuery(_ accountId: String) -> [String: Any] {
        var query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: Self.service,
            kSecAttrAccount as String: accountId,
            kSecAttrSynchronizable as String: kCFBooleanFalse as Any,
        ]
        #if os(macOS)
        // Access groups only apply to the data protection keychain on macOS.
        query[kSecUseDataProtectionKeychain as String] = true
        #endif
        return query
    }

    public func token(for accountId: String) throws -> String? {
        var query = baseQuery(accountId)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else { throw KeychainError(status: status) }
        return (result as? Data).flatMap { String(data: $0, encoding: .utf8) }
    }

    public func setToken(_ token: String, for accountId: String) throws {
        let data = Data(token.utf8)
        var add = baseQuery(accountId)
        add[kSecValueData as String] = data
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly

        var status = SecItemAdd(add as CFDictionary, nil)
        if status == errSecDuplicateItem {
            status = SecItemUpdate(
                baseQuery(accountId) as CFDictionary,
                [kSecValueData as String: data, kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly] as CFDictionary
            )
        }
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }

    public func deleteToken(for accountId: String) throws {
        let status = SecItemDelete(baseQuery(accountId) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else { throw KeychainError(status: status) }
    }
}

/// Vault that keeps tokens in memory, for tests and previews.
public final class InMemoryTokenVault: TokenVault, @unchecked Sendable {
    private let lock = NSLock()
    private var tokens: [String: String] = [:]

    public init() {}

    public func token(for accountId: String) throws -> String? {
        lock.withLock { tokens[accountId] }
    }

    public func setToken(_ token: String, for accountId: String) throws {
        lock.withLock { tokens[accountId] = token }
    }

    public func deleteToken(for accountId: String) throws {
        lock.withLock { _ = tokens.removeValue(forKey: accountId) }
    }
}
