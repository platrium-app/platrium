import XCTest
@testable import PlatriumCore

final class SharingFormatTests: XCTestCase {
    func testHumanizeRole() {
        XCTAssertEqual(SharingFormat.humanizeRole("FULL_EDITOR"), "Full Editor")
        XCTAssertEqual(SharingFormat.humanizeRole("VIEWER"), "Viewer")
        XCTAssertEqual(SharingFormat.humanizeRole("custom"), "Custom")
    }

    func testInitials() {
        XCTAssertEqual(SharingFormat.initials("Ada Lovelace"), "AL")
        XCTAssertEqual(SharingFormat.initials("Ada Byron King Lovelace"), "AL")
        XCTAssertEqual(SharingFormat.initials("plato"), "PL")
        XCTAssertEqual(SharingFormat.initials("  "), "?")
    }

    func testItemLinkMatchesTheWebRoutes() {
        XCTAssertEqual(SharingFormat.itemLink(serverURL: "https://files.example.com", id: "abc", isFolder: true)?.absoluteString, "https://files.example.com/folder/abc")
        XCTAssertEqual(SharingFormat.itemLink(serverURL: "http://10.0.0.2:3000", id: "x1", isFolder: false)?.absoluteString, "http://10.0.0.2:3000/file/x1")
    }

    func testFriendlyMessages() {
        XCTAssertEqual(SharingFormat.friendlyMessage(code: "FORBIDDEN", message: "your organization does not allow public sharing"), "Your organization does not allow public sharing.")
        XCTAssertEqual(SharingFormat.friendlyMessage(code: "FORBIDDEN", message: "nope"), "You don't have permission to do that.")
        XCTAssertEqual(SharingFormat.friendlyMessage(code: "NOT_FOUND", message: nil), "That item or person no longer exists.")
        XCTAssertEqual(SharingFormat.friendlyMessage(code: "UNAUTHENTICATED", message: nil), "Your session has ended. Sign in again.")
        XCTAssertEqual(SharingFormat.friendlyMessage(code: "CONFLICT", message: nil), "That already exists.")
        // Anything else surfaces the server's own words, which are written for people.
        XCTAssertEqual(SharingFormat.friendlyMessage(code: "BAD_REQUEST", message: "you cannot change your own access"), "you cannot change your own access")
        XCTAssertEqual(SharingFormat.friendlyMessage(code: nil, message: nil), "Something went wrong. Try again.")
    }

    func testDeniedMeansForbiddenOrNotFound() {
        XCTAssertTrue(SharingError(code: "FORBIDDEN", serverMessage: nil).isDenied)
        XCTAssertTrue(SharingError(code: "NOT_FOUND", serverMessage: nil).isDenied)
        XCTAssertFalse(SharingError(code: "BAD_REQUEST", serverMessage: nil).isDenied)
    }

    func testEndOfDay() {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "America/New_York")!
        let noon = calendar.date(from: DateComponents(year: 2026, month: 3, day: 9, hour: 12))!
        let end = SharingFormat.endOfDay(noon, calendar: calendar)
        let parts = calendar.dateComponents([.year, .month, .day, .hour, .minute, .second], from: end)
        XCTAssertEqual([parts.year, parts.month, parts.day, parts.hour, parts.minute, parts.second], [2026, 3, 9, 23, 59, 59])
    }

    func testDatesRoundTripWithAndWithoutFractions() throws {
        let plain = try XCTUnwrap(SharingFormat.parseDate("2030-01-02T03:04:05Z"))
        let fractional = try XCTUnwrap(SharingFormat.parseDate("2030-01-02T03:04:05.250Z"))
        XCTAssertEqual(fractional.timeIntervalSince(plain), 0.25, accuracy: 0.001)
        XCTAssertEqual(SharingFormat.isoString(plain), "2030-01-02T03:04:05Z")
        XCTAssertNil(SharingFormat.parseDate("not a date"))
        XCTAssertNil(SharingFormat.parseDate(nil))
    }

    func testCapabilities() {
        XCTAssertTrue(["VIEW", "SHARE"].can(Capability.share))
        XCTAssertFalse(["VIEW", "DOWNLOAD"].can(Capability.share))
        XCTAssertTrue(SharingFormat.isDriveRoot(parentId: nil))
        XCTAssertFalse(SharingFormat.isDriveRoot(parentId: "p"))
    }
}
