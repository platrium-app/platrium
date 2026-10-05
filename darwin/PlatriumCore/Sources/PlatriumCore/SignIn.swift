import Foundation
import PlatriumSDK

/// The two server calls the sign-in flow needs, so tests can stand in for the
/// network. The production implementation is `SDKAuthBackend`.
public protocol AuthBackend: Sendable {
    func exchangeCode(server: Server, code: String, verifier: String) async throws -> TokenGrant
    func identity(server: Server, token: String) async throws -> Identity
}

/// Talks to the engine through the Rust SDK, whose auth calls are generated
/// from the engine's OpenAPI, so an API change breaks the build here.
public struct SDKAuthBackend: AuthBackend {
    public init() {}

    public func exchangeCode(server: Server, code: String, verifier: String) async throws -> TokenGrant {
        try await PlatriumClient(baseUrl: server.restBaseURL).auth().exchangeCode(code: code, codeVerifier: verifier)
    }

    public func identity(server: Server, token: String) async throws -> Identity {
        try await PlatriumClient.withToken(baseUrl: server.restBaseURL, token: token).auth().me()
    }
}

public enum SignInError: LocalizedError {
    /// The user closed the browser sheet.
    case cancelled
    /// The callback did not carry the `state` we sent, so it is not ours.
    case stateMismatch
    /// The server (or the user) refused the request.
    case denied(String)
    case missingCode
    case invalidAuthorizeURL
    /// The server answered with an error, or could not be reached.
    case server(String)

    public var errorDescription: String? {
        switch self {
        case .cancelled: "Sign-in was cancelled."
        case .stateMismatch: "The sign-in response didn't match the request. Please try again."
        case .denied(let reason): "Sign-in was refused: \(reason)."
        case .missingCode: "The server didn't return a sign-in code."
        case .invalidAuthorizeURL: "Couldn't build the sign-in address for this server."
        case .server(let message): message
        }
    }
}

public struct SignInResult: Sendable {
    public var account: Account
    /// False when this user was already signed in on this server and the
    /// existing account got a fresh credential.
    public var isNewAccount: Bool
}

/// Opens `url` in a browser session and returns the URL it was redirected to
/// (the one starting with `callbackScheme`). Throw `SignInError.cancelled` if
/// the user dismisses it.
public typealias BrowserAuthenticator = @Sendable (_ url: URL, _ callbackScheme: String) async throws -> URL

/// The token grant: approve in the browser, trade the one-time code for a
/// bearer token, find out who it belongs to, and store it.
public struct SignInService: Sendable {
    public static let callbackScheme = "platrium"
    public static let redirectURI = "\(callbackScheme)://auth/callback"

    private let repository: AccountRepository
    private let vault: any TokenVault
    private let backend: any AuthBackend
    private let device: DeviceDescriptor

    public init(
        repository: AccountRepository,
        vault: any TokenVault,
        backend: any AuthBackend = SDKAuthBackend(),
        device: DeviceDescriptor = .current
    ) {
        self.repository = repository
        self.vault = vault
        self.backend = backend
        self.device = device
    }

    public func signIn(to server: Server, authenticate: BrowserAuthenticator) async throws -> SignInResult {
        let pkce = PKCE()
        let state = PKCE.randomState()

        guard let authorizeURL = authorizeURL(server: server, challenge: pkce.challenge, state: state) else {
            throw SignInError.invalidAuthorizeURL
        }
        let callback = try await authenticate(authorizeURL, Self.callbackScheme)
        let code = try Self.code(from: callback, expectedState: state)

        let grant: TokenGrant
        let identity: Identity
        do {
            grant = try await backend.exchangeCode(server: server, code: code, verifier: pkce.verifier)
            identity = try await backend.identity(server: server, token: grant.token)
        } catch let error as PlatriumError {
            throw SignInError.server(Self.describe(error))
        }

        // Re-signing in keeps the account's id: the Keychain item and the File
        // Provider domain are keyed by it.
        let existing = try repository.account(serverId: server.id, tenantId: identity.tenantId, userId: identity.userId)
        let accountId = existing?.id ?? UUID().uuidString

        // Keychain first: a row without a token is useless, a token without a
        // row is harmless and gets overwritten next time.
        try vault.setToken(grant.token, for: accountId)
        do {
            let account = try repository.saveAccount(
                id: accountId, serverId: server.id, tenantId: identity.tenantId, userId: identity.userId,
                email: identity.email, tokenId: grant.tokenId, deviceId: grant.deviceId
            )
            return SignInResult(account: account, isNewAccount: existing == nil)
        } catch {
            if existing == nil { try? vault.deleteToken(for: accountId) }
            throw error
        }
    }

    func authorizeURL(server: Server, challenge: String, state: String) -> URL? {
        guard var components = URLComponents(string: server.url + "/authorize") else { return nil }
        components.queryItems = [
            URLQueryItem(name: "redirect_uri", value: Self.redirectURI),
            URLQueryItem(name: "code_challenge", value: challenge),
            URLQueryItem(name: "state", value: state),
            URLQueryItem(name: "name", value: device.name),
            URLQueryItem(name: "platform", value: device.platform),
            URLQueryItem(name: "app_version", value: device.appVersion),
        ]
        return components.url
    }

    /// The SDK's errors print as enum dumps; say what happened instead.
    static func describe(_ error: PlatriumError) -> String {
        switch error {
        case .Unauthorized:
            "The server didn't accept the sign-in. Please try again."
        case .BadRequest:
            "The sign-in expired or was already used. Please try again."
        case .ApiError(let message):
            message.contains("HTTP 5")
                ? "The server ran into a problem while signing you in. Please try again, or contact your administrator."
                : "Couldn't complete sign-in: \(message)"
        case .InternalError(let message):
            "Couldn't complete sign-in: \(message)"
        }
    }

    static func code(from callback: URL, expectedState: String) throws -> String {
        let items = URLComponents(url: callback, resolvingAgainstBaseURL: false)?.queryItems ?? []
        func value(_ name: String) -> String? { items.first { $0.name == name }?.value }

        if let error = value("error") { throw SignInError.denied(value("error_description") ?? error) }
        guard value("state") == expectedState else { throw SignInError.stateMismatch }
        guard let code = value("code"), !code.isEmpty else { throw SignInError.missingCode }
        return code
    }
}
