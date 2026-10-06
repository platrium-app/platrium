import Apollo
import PlatriumGraphQL
import XCTest
@testable import PlatriumCore

/// Drives the real sharing API through `SharingService`, as the app's sharing
/// sheet does. Needs a second user on the server (`PLATRIUM_TEST_OTHER_EMAIL`)
/// to share with; without one only the single-user checks run.
final class SharingLiveTests: XCTestCase {
    private func newFolder(in session: LiveSession) async throws -> (rootId: String, folderId: String) {
        let drives = try await session.apollo.fetch(query: GetDrivesListQuery(), cachePolicy: .networkOnly)
        let root = try XCTUnwrap(drives.data?.driveNodes.first { $0.driveMetadata?.driveType.value == .private })
        let created = try await session.apollo.perform(mutation: FSECreateFolderMutation(parentId: root.id, name: "Shared \(UUID().uuidString.prefix(6))"))
        XCTAssertNil(created.errors)
        return (root.id, try XCTUnwrap(created.data?.createFolder.id))
    }

    func testReadingAccessAndOptions() async throws {
        let session = try await LiveSession.signIn(try LiveConfig.load())
        let sharing = SharingService(apollo: session.apollo)
        let (rootId, folderId) = try await newFolder(in: session)

        let snapshot = try await sharing.load(itemId: folderId)
        XCTAssertTrue(snapshot.access.owner.isYou)
        XCTAssertEqual(snapshot.access.owner.type, "USER")
        XCTAssertEqual(snapshot.access.general.level, "RESTRICTED")
        XCTAssertTrue(snapshot.access.grants.isEmpty)
        XCTAssertTrue(snapshot.access.inheritsPermissions)

        // A folder offers the two roles, with labels from the server.
        XCTAssertEqual(snapshot.roles.map(\.role), ["VIEWER", "FULL_EDITOR"])
        XCTAssertEqual(snapshot.roles.map(\.label), ["Viewer", "Editor"])
        XCTAssertEqual(snapshot.roles.first { $0.role == "VIEWER" }?.downloadOptional, true)
        XCTAssertTrue(snapshot.levels.map(\.level).starts(with: ["RESTRICTED", "TENANT"]))
        XCTAssertTrue(snapshot.levels.first { $0.level == "RESTRICTED" }?.roles.isEmpty ?? false)

        // A private drive's root can't be shared: the server hides it as denied.
        do {
            _ = try await sharing.access(itemId: rootId)
            XCTFail("a private drive root must not be shareable")
        } catch let error as SharingError {
            XCTAssertTrue(error.isDenied, "\(error)")
        }

        // An item that doesn't exist looks the same as one you may not see.
        do {
            _ = try await sharing.access(itemId: "does-not-exist")
            XCTFail("expected NOT_FOUND")
        } catch let error as SharingError {
            XCTAssertEqual(error.code, "NOT_FOUND")
            XCTAssertEqual(error.errorDescription, "That item or person no longer exists.")
        }
    }

    func testGeneralAccessAndInheritance() async throws {
        let session = try await LiveSession.signIn(try LiveConfig.load())
        let sharing = SharingService(apollo: session.apollo)
        let (_, folderId) = try await newFolder(in: session)

        try await sharing.setGeneralAccess(itemId: folderId, level: "TENANT", role: "VIEWER", noDownload: true, expiresAt: nil)
        var general = try await sharing.access(itemId: folderId).general
        XCTAssertEqual([general.level, general.role], ["TENANT", "VIEWER"])
        XCTAssertTrue(general.noDownload)

        // Expiry survives the round trip.
        let tomorrow = SharingFormat.endOfDay(Date().addingTimeInterval(86_400))
        try await sharing.setGeneralAccess(itemId: folderId, level: "TENANT", role: "FULL_EDITOR", noDownload: false, expiresAt: tomorrow)
        general = try await sharing.access(itemId: folderId).general
        XCTAssertEqual(general.role, "FULL_EDITOR")
        XCTAssertEqual(try XCTUnwrap(general.expiresAt).timeIntervalSince1970, tomorrow.timeIntervalSince1970, accuracy: 1)

        // A role the level doesn't offer is refused with the server's own explanation.
        do {
            try await sharing.setGeneralAccess(itemId: folderId, level: "PUBLIC", role: "FULL_EDITOR", noDownload: false, expiresAt: nil)
            XCTFail("public access is view-only")
        } catch let error as SharingError {
            XCTAssertNotNil(error.code)
            XCTAssertFalse((error.errorDescription ?? "").isEmpty)
        }

        try await sharing.setGeneralAccess(itemId: folderId, level: "RESTRICTED", role: nil, noDownload: false, expiresAt: nil)
        general = try await sharing.access(itemId: folderId).general
        XCTAssertEqual(general.level, "RESTRICTED")

        // A folder inherits by default, and can stop.
        try await sharing.setInheritance(itemId: folderId, inherit: false)
        var inherits = try await sharing.access(itemId: folderId).inheritsPermissions
        XCTAssertFalse(inherits)
        try await sharing.setInheritance(itemId: folderId, inherit: true)
        inherits = try await sharing.access(itemId: folderId).inheritsPermissions
        XCTAssertTrue(inherits)
    }

    func testSharingWithAnotherUser() async throws {
        let cfg = try LiveConfig.load()
        guard let otherEmail = cfg.otherEmail else { throw XCTSkip("Set PLATRIUM_TEST_OTHER_EMAIL to a second user to test sharing with someone.") }
        let session = try await LiveSession.signIn(cfg)
        let sharing = SharingService(apollo: session.apollo)
        let (_, folderId) = try await newFolder(in: session)

        // The people picker finds them, but never offers yourself.
        let found = try await sharing.search(String(otherEmail.prefix(4)), excludingAccessTo: folderId)
        let other = try XCTUnwrap(found.first { $0.email?.lowercased() == otherEmail.lowercased() }, "\(found)")
        XCTAssertEqual(other.type, "USER")
        XCTAssertFalse(found.contains { $0.email?.lowercased() == cfg.email.lowercased() })
        let tooShort = try await sharing.search("x", excludingAccessTo: folderId)
        XCTAssertTrue(tooShort.isEmpty, "queries under two characters return nothing")

        // Share, then they no longer appear in search for this item.
        try await sharing.share(itemId: folderId, subjectType: other.type, subjectId: other.personId, role: "VIEWER")
        var access = try await sharing.access(itemId: folderId)
        XCTAssertEqual(access.grants.count, 1)
        let grant = try XCTUnwrap(access.grants.first)
        XCTAssertEqual([grant.subjectId, grant.role], [other.personId, "VIEWER"])
        let afterShare = try await sharing.search(String(otherEmail.prefix(4)), excludingAccessTo: folderId)
        XCTAssertFalse(afterShare.contains { $0.personId == other.personId })

        // Changing a role is sharing again: still one grant.
        try await sharing.share(itemId: folderId, subjectType: other.type, subjectId: other.personId, role: "FULL_EDITOR")
        access = try await sharing.access(itemId: folderId)
        XCTAssertEqual(access.grants.map(\.role), ["FULL_EDITOR"])

        // They can open it as that role.
        let theirs = try await LiveSession.signIn(cfg, email: otherEmail, password: ProcessInfo.processInfo.environment["PLATRIUM_TEST_OTHER_PASSWORD"] ?? cfg.password)
        let theirInfo = try await theirs.apollo.fetch(query: GetDriveItemInfoQuery(id: folderId), cachePolicy: .networkOnly)
        XCTAssertTrue(theirInfo.data?.item?.myCapabilities.contains("EDIT") ?? false)
        XCTAssertFalse(theirInfo.data?.item?.myCapabilities.can(Capability.share) ?? true, "an editor can't manage access")

        // Revoke.
        try await sharing.revoke(grantId: grant.id)
        access = try await sharing.access(itemId: folderId)
        XCTAssertTrue(access.grants.isEmpty)

        // You can't change your own access.
        do {
            try await sharing.share(itemId: folderId, subjectType: "USER", subjectId: session.account.remoteUserId, role: "VIEWER")
            XCTFail("self-edit must be refused")
        } catch let error as SharingError {
            XCTAssertEqual(error.code, "BAD_REQUEST")
        }
    }
}
