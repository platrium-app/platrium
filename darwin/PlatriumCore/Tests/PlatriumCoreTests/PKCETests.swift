import XCTest
@testable import PlatriumCore

final class PKCETests: XCTestCase {
    func testChallengeMatchesRFC7636Vector() {
        // RFC 7636, Appendix B.
        let pkce = PKCE(verifier: "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
        XCTAssertEqual(pkce.challenge, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM")
    }

    func testGeneratedPairIsValidForTheEngine() {
        let pkce = PKCE()
        // The engine requires a 43...128 character verifier and a 32-byte (43 character) challenge.
        XCTAssertEqual(pkce.verifier.count, 43)
        XCTAssertEqual(pkce.challenge.count, 43)
        XCTAssertEqual(pkce.challenge, PKCE.challenge(for: pkce.verifier))
        let alphabet = CharacterSet(charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_")
        XCTAssertTrue(pkce.verifier.unicodeScalars.allSatisfy(alphabet.contains))
    }

    func testPairsAreUnique() {
        XCTAssertNotEqual(PKCE().verifier, PKCE().verifier)
        XCTAssertNotEqual(PKCE.randomState(), PKCE.randomState())
    }
}
