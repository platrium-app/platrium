import XCTest
@testable import PlatriumCore

final class ServerURLTests: XCTestCase {
    func testNormalizesCommonInput() throws {
        XCTAssertEqual(try ServerURL.normalize("https://Files.Example.com/"), "https://files.example.com")
        XCTAssertEqual(try ServerURL.normalize("  https://files.example.com//  "), "https://files.example.com")
        XCTAssertEqual(try ServerURL.normalize("https://example.com/platrium/?x=1#frag"), "https://example.com/platrium")
        XCTAssertEqual(try ServerURL.normalize("http://172.20.0.179:3000"), "http://172.20.0.179:3000")
    }

    func testMissingSchemeDefaultsToHTTPSForPublicHostsAndHTTPForLocalOnes() throws {
        XCTAssertEqual(try ServerURL.normalize("files.example.com"), "https://files.example.com")
        XCTAssertEqual(try ServerURL.normalize("localhost:3000"), "http://localhost:3000")
        XCTAssertEqual(try ServerURL.normalize("192.168.1.20:3000"), "http://192.168.1.20:3000")
        XCTAssertEqual(try ServerURL.normalize("nas.local"), "http://nas.local")
        XCTAssertEqual(try ServerURL.normalize("nas"), "http://nas")
    }

    func testRejectsBadInput() {
        XCTAssertThrowsError(try ServerURL.normalize("   ")) { XCTAssertEqual($0 as? ServerURLError, .empty) }
        XCTAssertThrowsError(try ServerURL.normalize("ftp://example.com")) {
            XCTAssertEqual($0 as? ServerURLError, .unsupportedScheme("ftp"))
        }
        XCTAssertThrowsError(try ServerURL.normalize("https://user:pw@example.com")) { XCTAssertEqual($0 as? ServerURLError, .invalid) }
        XCTAssertThrowsError(try ServerURL.normalize("https://")) { XCTAssertEqual($0 as? ServerURLError, .invalid) }
    }
}
