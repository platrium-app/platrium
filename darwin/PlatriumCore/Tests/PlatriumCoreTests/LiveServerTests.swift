import Apollo
import PlatriumGraphQL
import PlatriumSDK
import XCTest
@testable import PlatriumCore

/// Runs the real sign-in flow against a running engine, with the browser played
/// by a REST session. Skipped unless a server is configured:
///
///     PLATRIUM_TEST_SERVER=http://localhost:3999 PLATRIUM_TEST_EMAIL=... PLATRIUM_TEST_PASSWORD=... swift test --filter LiveServerTests
///
/// It covers what the unit tests fake: the Rust SDK's auth calls, the typed
/// `serverInfo` probe, the bearer interceptor, and what the server does when a
/// device is revoked.
final class LiveServerTests: XCTestCase {
    private struct Config {
        let server: String
        let email: String
        let password: String
    }

    private func config() throws -> Config {
        let env = ProcessInfo.processInfo.environment
        guard let server = env["PLATRIUM_TEST_SERVER"], let email = env["PLATRIUM_TEST_EMAIL"], let password = env["PLATRIUM_TEST_PASSWORD"] else {
            throw XCTSkip("Set PLATRIUM_TEST_SERVER, PLATRIUM_TEST_EMAIL and PLATRIUM_TEST_PASSWORD to run against a live engine.")
        }
        return Config(server: server, email: email, password: password)
    }

    /// A browser: keeps cookies, signs in with a password, and approves clients.
    private final class FakeBrowser: @unchecked Sendable {
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

    private final class Flag: @unchecked Sendable {
        private let lock = NSLock()
        private var count = 0
        var value: Int { lock.withLock { count } }
        func bump() { lock.withLock { count += 1 } }
    }

    func testSignInThroughRevocationAndSignInAgain() async throws {
        let cfg = try config()
        let normalized = try ServerURL.normalize(cfg.server)

        // The typed, unauthenticated probe works against a real server.
        let details = try await ServerProbe.fetch(serverURL: normalized)
        XCTAssertFalse(details.version.isEmpty)

        let repository = AccountRepository(database: try AppDatabase.inMemory())
        let server = try repository.addServer(url: normalized)
        let vault = InMemoryTokenVault()
        let device = DeviceDescriptor(platform: "MACOS", name: "Platrium on Test Runner", appVersion: "0.0.1")
        let service = SignInService(repository: repository, vault: vault, backend: SDKAuthBackend(), device: device)

        let browser = FakeBrowser(base: normalized)
        try await browser.logIn(email: cfg.email, password: cfg.password)

        // 1. Sign in: authorize in the "browser", exchange the code through the Rust SDK.
        let first = try await service.signIn(to: server) { url, _ in try await browser.approve(authorizeURL: url) }
        XCTAssertTrue(first.isNewAccount)
        XCTAssertEqual(first.account.email, cfg.email)
        XCTAssertNotNil(first.account.deviceId, "a platform in the authorize request registers a device")
        let token = try XCTUnwrap(try vault.token(for: first.account.id))
        XCTAssertTrue(token.hasPrefix("plt_"))

        // 2. The Rust SDK client authenticates as a device.
        let sdk = try ClientFactory.makeSDK(server: server, account: first.account, vault: vault)
        let me = try await sdk.auth().me()
        XCTAssertEqual(me.kind, .device)
        XCTAssertEqual(me.deviceId, first.account.deviceId)
        XCTAssertEqual(me.email, cfg.email)

        // 3. The GraphQL client sends the bearer token: an authenticated query returns data.
        let unauthorized = Flag()
        let apollo = try ClientFactory.makeApollo(server: server, account: first.account, vault: vault) { _ in unauthorized.bump() }
        let drives = try await apollo.fetch(query: GetDrivesListQuery(), cachePolicy: .networkOnly)
        XCTAssertNil(drives.errors, "\(String(describing: drives.errors))")
        XCTAssertFalse(drives.data?.driveNodes.isEmpty ?? true, "the signed-in user owns at least a private drive")
        XCTAssertEqual(unauthorized.value, 0)

        // 4. Revoking the device on the server ends both clients' access.
        let revoked = try await browser.delete("/api/auth/clients/\(first.account.tokenId)")
        XCTAssertEqual(revoked, 204)
        do {
            _ = try await sdk.auth().me()
            XCTFail("a revoked token must not work")
        } catch PlatriumError.Unauthorized {}
        do {
            _ = try await apollo.fetch(query: GetDrivesListQuery(), cachePolicy: .networkOnly)
            XCTFail("a revoked token must not work over GraphQL")
        } catch {}
        XCTAssertEqual(unauthorized.value, 1, "the interceptor reported the 401")

        // 5. Signing in again keeps the same account (and so its Keychain item and domain).
        try repository.setStatus(.needsReauth, forAccount: first.account.id)
        let second = try await service.signIn(to: server) { url, _ in try await browser.approve(authorizeURL: url) }
        XCTAssertFalse(second.isNewAccount)
        XCTAssertEqual(second.account.id, first.account.id)
        XCTAssertEqual(second.account.status, .active)
        XCTAssertNotEqual(second.account.tokenId, first.account.tokenId)
        XCTAssertNotEqual(try vault.token(for: second.account.id), token)

        // 6. The fresh token works with the same Apollo client: it reads the vault on each request.
        let again = try await apollo.fetch(query: GetDrivesListQuery(), cachePolicy: .networkOnly)
        XCTAssertNil(again.errors)
        XCTAssertFalse(again.data?.driveNodes.isEmpty ?? true)
    }

    func testWrongVerifierIsRejectedByTheServer() async throws {
        let cfg = try config()
        let normalized = try ServerURL.normalize(cfg.server)
        let server = Server(name: "live", url: normalized)

        let browser = FakeBrowser(base: normalized)
        try await browser.logIn(email: cfg.email, password: cfg.password)

        let pkce = PKCE()
        let state = "s"
        var components = URLComponents(string: normalized + "/authorize")!
        components.queryItems = [
            .init(name: "redirect_uri", value: "platrium://auth/callback"),
            .init(name: "code_challenge", value: pkce.challenge),
            .init(name: "state", value: state),
            .init(name: "name", value: "Attacker"),
        ]
        let callback = try await browser.approve(authorizeURL: components.url!)
        let code = try SignInService.code(from: callback, expectedState: state)

        // Someone who intercepted the code does not have the verifier.
        do {
            _ = try await SDKAuthBackend().exchangeCode(server: server, code: code, verifier: PKCE().verifier)
            XCTFail("a code must not redeem without its verifier")
        } catch PlatriumError.BadRequest {}
    }
}
