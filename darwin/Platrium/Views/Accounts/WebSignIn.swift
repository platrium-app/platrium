import AuthenticationServices
import PlatriumCore
import SwiftUI

extension WebAuthenticationSession {
    /// Runs the sign-in page in an ephemeral browser session, so it never reuses
    /// Safari's cookies. That is what lets a second user sign in on the same server.
    var browserAuthenticator: BrowserAuthenticator {
        { url, scheme in
            do {
                return try await authenticate(
                    using: url,
                    callback: .customScheme(scheme),
                    preferredBrowserSession: .ephemeral,
                    additionalHeaderFields: [:]
                )
            } catch let error as ASWebAuthenticationSessionError where error.code == .canceledLogin {
                throw SignInError.cancelled
            }
        }
    }
}
