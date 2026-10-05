import XCTest
@testable import PlatriumCore

final class TokenVaultTests: XCTestCase {
    func testInMemoryVaultRoundTripAndIsolation() throws {
        let vault = InMemoryTokenVault()
        XCTAssertNil(try vault.token(for: "a"))
        try vault.setToken("one", for: "a")
        try vault.setToken("two", for: "b")
        try vault.setToken("uno", for: "a")
        XCTAssertEqual(try vault.token(for: "a"), "uno")
        XCTAssertEqual(try vault.token(for: "b"), "two")
        try vault.deleteToken(for: "a")
        XCTAssertNil(try vault.token(for: "a"))
        XCTAssertEqual(try vault.token(for: "b"), "two")
    }
}
