import GRDB
import XCTest
@testable import PlatriumCore

final class AccountRepositoryTests: XCTestCase {
    private var database: AppDatabase!
    private var repo: AccountRepository!

    override func setUpWithError() throws {
        database = try AppDatabase.inMemory()
        repo = AccountRepository(database: database)
    }

    /// SQLite keeps milliseconds, so compare what identifies a row, not its timestamps.
    private func ids(_ s: Selection) -> [String?] { [s.server?.id, s.account?.id] }

    private func save(_ server: Server, tenant: String = "t1", user: String, email: String? = nil, id: String = UUID().uuidString) throws -> Account {
        try repo.saveAccount(
            id: id, serverId: server.id, tenantId: tenant, userId: user,
            email: email ?? "\(user)@example.com", tokenId: "tok-\(user)", deviceId: "dev-\(user)"
        )
    }

    func testAddServerIsIdempotentAndNormalizes() throws {
        let a = try repo.addServer(url: "Example.com/")
        let b = try repo.addServer(name: "Other name", url: "https://example.com")
        XCTAssertEqual(a.id, b.id)
        XCTAssertEqual(a.url, "https://example.com")
        XCTAssertEqual(a.name, "example.com", "name defaults to the host")
        XCTAssertEqual(try repo.servers().count, 1)
    }

    func testManyUsersOnOneServerAndOneUserOnManyServers() throws {
        let one = try repo.addServer(url: "https://one.example.com")
        let two = try repo.addServer(url: "https://two.example.com")
        let alice1 = try save(one, user: "alice")
        _ = try save(one, user: "bob")
        let alice2 = try save(two, user: "alice")

        XCTAssertEqual(try repo.accounts(serverId: one.id).count, 2)
        XCTAssertEqual(try repo.accounts(serverId: two.id).count, 1)
        XCTAssertNotEqual(alice1.id, alice2.id, "the same user on two servers is two accounts")
    }

    func testSigningInAgainKeepsTheAccountIdAndReactivates() throws {
        let server = try repo.addServer(url: "https://one.example.com")
        let first = try save(server, user: "alice", id: "original-id")
        try repo.setStatus(.needsReauth, forAccount: first.id)

        let again = try repo.saveAccount(
            id: "a-different-id", serverId: server.id, tenantId: "t1", userId: "alice",
            email: "alice@new.example.com", tokenId: "tok-new", deviceId: "dev-new"
        )

        XCTAssertEqual(again.id, "original-id")
        XCTAssertEqual(again.status, .active)
        XCTAssertEqual(again.tokenId, "tok-new")
        XCTAssertEqual(again.email, "alice@new.example.com")
        XCTAssertEqual(try repo.accounts().count, 1)
    }

    func testSameUserInDifferentTenantsIsADifferentAccount() throws {
        let server = try repo.addServer(url: "https://one.example.com")
        _ = try save(server, tenant: "t1", user: "alice")
        _ = try save(server, tenant: "t2", user: "alice")
        XCTAssertEqual(try repo.accounts().count, 2)
    }

    func testAccountLookupByDomainIdentifier() throws {
        let server = try repo.addServer(url: "https://one.example.com")
        let account = try save(server, user: "alice", id: "domain-123")
        XCTAssertEqual(try repo.account(forDomain: "domain-123")?.id, account.id)
        XCTAssertNil(try repo.account(forDomain: "platrium_dev_abcd"))
    }

    func testDeletingAServerCascadesToItsAccounts() throws {
        let one = try repo.addServer(url: "https://one.example.com")
        let two = try repo.addServer(url: "https://two.example.com")
        _ = try save(one, user: "alice")
        let kept = try save(two, user: "bob")

        try database.writer.write { db in
            _ = try Server.deleteOne(db, key: one.id)
        }
        XCTAssertEqual(try repo.accounts().map(\.id), [kept.id])
    }

    func testSelectionStartsEmpty() throws {
        XCTAssertEqual(ids(try repo.selection()), [nil, nil])
    }

    func testSelectionFallsBackStepByStep() throws {
        let one = try repo.addServer(url: "https://one.example.com")
        let two = try repo.addServer(url: "https://two.example.com")

        // A server with no accounts can be selected.
        XCTAssertEqual(try repo.selection().server?.id, one.id, "first server when nothing is stored")
        try repo.setActive(serverId: two.id)
        XCTAssertEqual(ids(try repo.selection()), [two.id, nil])

        let alice = try save(one, user: "alice")
        let bob = try save(one, user: "bob")
        let carol = try save(two, user: "carol")

        // Stored server, no stored account: its first account.
        XCTAssertEqual(ids(try repo.selection()), [two.id, carol.id])

        try repo.setActive(accountId: bob.id)
        XCTAssertEqual(ids(try repo.selection()), [one.id, bob.id])
        XCTAssertEqual(try repo.activeServerId(), one.id, "selecting an account also selects its server")

        // Selecting the other server drops an active account that isn't on it.
        try repo.setActive(serverId: two.id)
        XCTAssertEqual(ids(try repo.selection()), [two.id, carol.id])

        // The stored account disappears: fall back instead of pointing at nothing.
        try repo.setActive(accountId: alice.id)
        try database.writer.write { db in _ = try Account.deleteOne(db, key: alice.id) }
        XCTAssertEqual(ids(try repo.selection()), [one.id, bob.id], "first account of the stored server")
    }

    func testSetActiveIgnoresUnknownIds() throws {
        try repo.setActive(accountId: "nope")
        try repo.setActive(serverId: "nope")
        XCTAssertNil(try repo.activeAccountId())
        XCTAssertNil(try repo.activeServerId())
    }
}
