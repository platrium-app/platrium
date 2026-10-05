import PlatriumSDK
import XCTest
@testable import PlatriumCore

private final class FakeBackend: AuthBackend, @unchecked Sendable {
    var identity = Identity(userId: "u1", tenantId: "t1", email: "alice@example.com", kind: .device, deviceId: "dev1")
    var grant = TokenGrant(token: "plt_first", tokenId: "tok1", deviceId: "dev1", expiresAt: nil)
    var exchangeError: Error?
    private(set) var exchanged: [(code: String, verifier: String)] = []
    private(set) var identityTokens: [String] = []

    func exchangeCode(server: Server, code: String, verifier: String) async throws -> TokenGrant {
        exchanged.append((code, verifier))
        if let exchangeError { throw exchangeError }
        return grant
    }

    func identity(server: Server, token: String) async throws -> Identity {
        identityTokens.append(token)
        return identity
    }
}

final class SignInServiceTests: XCTestCase {
    private var repo: AccountRepository!
    private var vault: InMemoryTokenVault!
    private var backend: FakeBackend!
    private var server: Server!
    private var service: SignInService!

    private let device = DeviceDescriptor(platform: "MACOS", name: "Platrium on Test Mac", appVersion: "1.2")

    override func setUpWithError() throws {
        repo = AccountRepository(database: try AppDatabase.inMemory())
        vault = InMemoryTokenVault()
        backend = FakeBackend()
        server = try repo.addServer(url: "https://one.example.com")
        service = SignInService(repository: repo, vault: vault, backend: backend, device: device)
    }

    /// Plays the browser: approves, and sends back `code` with the state the app asked for.
    private func approving(code: String = "the-code", captured: @escaping @Sendable (URL) -> Void = { _ in }) -> BrowserAuthenticator {
        { url, scheme in
            captured(url)
            let state = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems?.first { $0.name == "state" }?.value ?? ""
            return URL(string: "\(scheme)://auth/callback?code=\(code)&state=\(state)")!
        }
    }

    func testHappyPathStoresTokenInVaultAndAccountInDatabase() async throws {
        let result = try await service.signIn(to: server, authenticate: approving())

        XCTAssertTrue(result.isNewAccount)
        let account = result.account
        XCTAssertEqual(account.email, "alice@example.com")
        XCTAssertEqual(account.remoteTenantId, "t1")
        XCTAssertEqual(account.remoteUserId, "u1")
        XCTAssertEqual(account.tokenId, "tok1")
        XCTAssertEqual(account.deviceId, "dev1")
        XCTAssertEqual(account.status, .active)
        XCTAssertEqual(try vault.token(for: account.id), "plt_first")
        XCTAssertEqual(try repo.account(id: account.id)?.id, account.id)
        XCTAssertEqual(backend.exchanged.first?.code, "the-code")
        XCTAssertEqual(backend.identityTokens, ["plt_first"], "identity is asked for with the new token")
    }

    func testAuthorizeURLCarriesTheDeviceAndAPKCEChallenge() async throws {
        let box = LockedBox<URL>()
        _ = try await service.signIn(to: server, authenticate: approving { box.set($0) })

        let url = try XCTUnwrap(box.value)
        let items = Dictionary(uniqueKeysWithValues: (URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems ?? []).map { ($0.name, $0.value ?? "") })
        XCTAssertEqual(url.host(), "one.example.com")
        XCTAssertEqual(url.path(), "/authorize")
        XCTAssertEqual(items["redirect_uri"], "platrium://auth/callback")
        XCTAssertEqual(items["name"], "Platrium on Test Mac")
        XCTAssertEqual(items["platform"], "MACOS")
        XCTAssertEqual(items["app_version"], "1.2")
        // The verifier is not in the URL, but what was sent to the server must hash to the challenge.
        XCTAssertEqual(PKCE.challenge(for: try XCTUnwrap(backend.exchanged.first?.verifier)), items["code_challenge"])
        XCTAssertNil(items["code_verifier"])
    }

    func testStateMismatchIsRejectedBeforeAnyNetworkCall() async throws {
        let hostile: BrowserAuthenticator = { _, scheme in URL(string: "\(scheme)://auth/callback?code=stolen&state=wrong")! }
        do {
            _ = try await service.signIn(to: server, authenticate: hostile)
            XCTFail("expected a state mismatch")
        } catch SignInError.stateMismatch {}
        XCTAssertTrue(backend.exchanged.isEmpty)
        XCTAssertTrue(try repo.accounts().isEmpty)
    }

    func testErrorCallbackIsReportedAndNothingIsStored() async throws {
        let denied: BrowserAuthenticator = { _, scheme in URL(string: "\(scheme)://auth/callback?error=access_denied")! }
        do {
            _ = try await service.signIn(to: server, authenticate: denied)
            XCTFail("expected a denial")
        } catch SignInError.denied(let reason) {
            XCTAssertEqual(reason, "access_denied")
        }
        XCTAssertTrue(try repo.accounts().isEmpty)
    }

    func testMissingCode() async throws {
        let empty: BrowserAuthenticator = { url, scheme in
            let state = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems?.first { $0.name == "state" }?.value ?? ""
            return URL(string: "\(scheme)://auth/callback?state=\(state)")!
        }
        do {
            _ = try await service.signIn(to: server, authenticate: empty)
            XCTFail("expected a missing code")
        } catch SignInError.missingCode {}
    }

    func testCancellationPropagates() async throws {
        let cancelled: BrowserAuthenticator = { _, _ in throw SignInError.cancelled }
        do {
            _ = try await service.signIn(to: server, authenticate: cancelled)
            XCTFail("expected cancellation")
        } catch SignInError.cancelled {}
        XCTAssertTrue(try repo.accounts().isEmpty)
    }

    func testFailedExchangeStoresNothing() async throws {
        struct Boom: Error {}
        backend.exchangeError = Boom()
        do {
            _ = try await service.signIn(to: server, authenticate: approving())
            XCTFail("expected the exchange to fail")
        } catch is Boom {}
        XCTAssertTrue(try repo.accounts().isEmpty)
    }

    func testServerFailureBecomesAReadableMessage() async throws {
        backend.exchangeError = PlatriumError.ApiError("HTTP 500 Internal Server Error: {\"debuginfo\":\"failed to issue token\"}\n")
        do {
            _ = try await service.signIn(to: server, authenticate: approving())
            XCTFail("expected a server error")
        } catch let error as SignInError {
            let message = try XCTUnwrap(error.errorDescription)
            XCTAssertFalse(message.contains("PlatriumError"), message)
            XCTAssertFalse(message.contains("debuginfo"), message)
            XCTAssertTrue(message.contains("server"), message)
        }
        XCTAssertTrue(try repo.accounts().isEmpty)
    }

    func testSigningInAgainReplacesTheTokenAndKeepsOneAccount() async throws {
        let first = try await service.signIn(to: server, authenticate: approving())
        try repo.setStatus(.needsReauth, forAccount: first.account.id)

        backend.grant = TokenGrant(token: "plt_second", tokenId: "tok2", deviceId: "dev2", expiresAt: nil)
        let second = try await service.signIn(to: server, authenticate: approving())

        XCTAssertFalse(second.isNewAccount)
        XCTAssertEqual(second.account.id, first.account.id, "the id keys the Keychain item and File Provider domain")
        XCTAssertEqual(second.account.status, .active)
        XCTAssertEqual(second.account.tokenId, "tok2")
        XCTAssertEqual(try vault.token(for: first.account.id), "plt_second")
        XCTAssertEqual(try repo.accounts().count, 1)
    }

    func testSecondUserOnTheSameServerIsASeparateAccount() async throws {
        let alice = try await service.signIn(to: server, authenticate: approving())

        backend.identity = Identity(userId: "u2", tenantId: "t1", email: "bob@example.com", kind: .device, deviceId: "dev2")
        backend.grant = TokenGrant(token: "plt_bob", tokenId: "tok-bob", deviceId: "dev2", expiresAt: nil)
        let bob = try await service.signIn(to: server, authenticate: approving())

        XCTAssertTrue(bob.isNewAccount)
        XCTAssertNotEqual(alice.account.id, bob.account.id)
        XCTAssertEqual(try vault.token(for: alice.account.id), "plt_first")
        XCTAssertEqual(try vault.token(for: bob.account.id), "plt_bob")
    }
}

private final class LockedBox<T>: @unchecked Sendable {
    private let lock = NSLock()
    private var stored: T?
    var value: T? { lock.withLock { stored } }
    func set(_ v: T) { lock.withLock { stored = v } }
}
