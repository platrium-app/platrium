import Apollo
import XCTest
@testable import PlatriumCore

/// Where the live tests find their server. They are skipped when it is not set:
///
///     PLATRIUM_TEST_SERVER=http://localhost:3999 PLATRIUM_TEST_EMAIL=... PLATRIUM_TEST_PASSWORD=... swift test --filter Live
struct LiveConfig {
    let server: String
    let email: String
    let password: String
    /// A second user on the same server, for sharing tests. Optional.
    let otherEmail: String?

    static func load() throws -> LiveConfig {
        let env = ProcessInfo.processInfo.environment
        guard let server = env["PLATRIUM_TEST_SERVER"], let email = env["PLATRIUM_TEST_EMAIL"], let password = env["PLATRIUM_TEST_PASSWORD"] else {
            throw XCTSkip("Set PLATRIUM_TEST_SERVER, PLATRIUM_TEST_EMAIL and PLATRIUM_TEST_PASSWORD to run against a live engine.")
        }
        return LiveConfig(server: server, email: email, password: password, otherEmail: env["PLATRIUM_TEST_OTHER_EMAIL"])
    }
}

/// A browser: keeps cookies, signs in with a password, and approves clients.
final class FakeBrowser: @unchecked Sendable {
    let session = URLSession(configuration: .ephemeral)
    let base: String

    init(base: String) { self.base = base }

    func post(_ path: String, _ body: [String: Any]) async throws -> (Int, [String: Any]) {
        var request = URLRequest(url: URL(string: base + path)!)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: body)
        return try await send(request)
    }

    func delete(_ path: String) async throws -> Int {
        var request = URLRequest(url: URL(string: base + path)!)
        request.httpMethod = "DELETE"
        return try await send(request).0
    }

    private func send(_ request: URLRequest) async throws -> (Int, [String: Any]) {
        let (data, response) = try await session.data(for: request)
        let json = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] ?? [:]
        return ((response as! HTTPURLResponse).statusCode, json)
    }

    func logIn(email: String, password: String) async throws {
        let idpId = try await localIdpId()
        let (status, body) = try await post("/api/auth/login", ["idp_id": idpId, "email": email, "password": password])
        XCTAssertEqual(status, 200, "\(body)")
    }

    private func localIdpId() async throws -> String {
        let (_, body) = try await post("/graphql", ["query": "{ tenantAuthConfig { providers { id type } } }"])
        let providers = ((body["data"] as? [String: Any])?["tenantAuthConfig"] as? [String: Any])?["providers"] as? [[String: Any]] ?? []
        return try XCTUnwrap(providers.first { $0["type"] as? String == "LOCAL" }?["id"] as? String)
    }

    /// What `ASWebAuthenticationSession` would do: show the consent page at
    /// the authorize URL and, when the user presses Allow, follow the redirect.
    func approve(authorizeURL: URL) async throws -> URL {
        let items = URLComponents(url: authorizeURL, resolvingAgainstBaseURL: false)?.queryItems ?? []
        var body: [String: Any] = [:]
        for item in items { body[item.name] = item.value }
        let (status, response) = try await post("/api/auth/authorize", body)
        XCTAssertEqual(status, 200, "\(response)")
        return URL(string: try XCTUnwrap(response["redirect_to"] as? String))!
    }
}

final class Flag: @unchecked Sendable {
    private let lock = NSLock()
    private var count = 0
    var value: Int { lock.withLock { count } }
    func bump() { lock.withLock { count += 1 } }
}

/// A user signed in to a live server through the real flow, with a GraphQL client.
struct LiveSession {
    let server: Server
    let account: Account
    let apollo: ApolloClient
    let browser: FakeBrowser

    static func signIn(_ cfg: LiveConfig, email: String? = nil, password: String? = nil) async throws -> LiveSession {
        let normalized = try ServerURL.normalize(cfg.server)
        let repository = AccountRepository(database: try AppDatabase.inMemory())
        let server = try repository.addServer(url: normalized)
        let vault = InMemoryTokenVault()
        let service = SignInService(
            repository: repository, vault: vault, backend: SDKAuthBackend(),
            device: DeviceDescriptor(platform: "MACOS", name: "Platrium on Test Runner", appVersion: "0.0.1")
        )
        let browser = FakeBrowser(base: normalized)
        try await browser.logIn(email: email ?? cfg.email, password: password ?? cfg.password)
        let result = try await service.signIn(to: server) { url, _ in try await browser.approve(authorizeURL: url) }
        let apollo = try ClientFactory.makeApollo(server: server, account: result.account, vault: vault) { _ in }
        return LiveSession(server: server, account: result.account, apollo: apollo, browser: browser)
    }
}
