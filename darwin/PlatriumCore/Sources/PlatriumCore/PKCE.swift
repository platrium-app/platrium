import CryptoKit
import Foundation
import Security

/// A PKCE (RFC 7636) verifier and its S256 challenge. The verifier never
/// leaves the device until the code exchange, which is what makes a stolen
/// authorization code useless.
public struct PKCE: Sendable, Equatable {
    public let verifier: String
    public let challenge: String

    /// A fresh random pair: 32 random bytes, which is 43 base64url characters.
    public init() {
        self.init(verifier: Self.randomToken())
    }

    public init(verifier: String) {
        self.verifier = verifier
        self.challenge = Self.challenge(for: verifier)
    }

    public static func challenge(for verifier: String) -> String {
        Data(SHA256.hash(data: Data(verifier.utf8))).base64URLEncodedString()
    }

    /// Unguessable value for the OAuth `state` parameter.
    public static func randomState() -> String { randomToken() }

    private static func randomToken() -> String {
        var bytes = [UInt8](repeating: 0, count: 32)
        let status = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        precondition(status == errSecSuccess, "system random number generator failed")
        return Data(bytes).base64URLEncodedString()
    }
}

extension Data {
    func base64URLEncodedString() -> String {
        base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}
