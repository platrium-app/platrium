import PlatriumCore
import SwiftUI

/// Shown in place of the content when the server no longer accepts the active
/// account's token (it was revoked or expired).
struct SignedOutView: View {
    @Environment(AccountStore.self) private var store
    @Environment(\.webAuthenticationSession) private var webAuthenticationSession

    let account: Account
    let server: Server

    @State private var isWorking = false
    @State private var errorMessage: String?

    var body: some View {
        ContentUnavailableView {
            Label("Signed Out", systemImage: "person.crop.circle.badge.exclamationmark")
        } description: {
            Text("Your session for \(account.email) on \(server.host) has ended.")
            if let errorMessage {
                Text(errorMessage).foregroundStyle(.red)
            }
        } actions: {
            Button {
                Task { await signIn() }
            } label: {
                if isWorking { ProgressView().controlSize(.small) } else { Text("Sign In") }
            }
            .buttonStyle(.borderedProminent)
            .disabled(isWorking)

            Button("Switch Account") { store.isShowingAccountSwitcher = true }
        }
    }

    private func signIn() async {
        isWorking = true
        errorMessage = nil
        defer { isWorking = false }
        do {
            try await store.signIn(to: server, authenticate: webAuthenticationSession.browserAuthenticator)
        } catch SignInError.cancelled {
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
