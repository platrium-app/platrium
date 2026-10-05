import PlatriumCore
import SwiftUI

struct SettingsView: View {
    @Environment(AccountStore.self) private var store

    var body: some View {
        Form {
            if let account = store.activeAccount, let server = store.activeServer {
                Section("Account") {
                    LabeledContent("Email", value: account.email)
                    LabeledContent("Server", value: server.host)
                    if account.status == .needsReauth {
                        Label("Sign in again to continue", systemImage: "exclamationmark.triangle")
                            .foregroundStyle(.red)
                    }
                }
            }

            Section("This Device") {
                LabeledContent("Name", value: DeviceDescriptor.current.name)
                LabeledContent("Version", value: DeviceDescriptor.current.appVersion)
            }

            Section {
                Button("Switch Account…") { store.isShowingAccountSwitcher = true }
            }
        }
        .formStyle(.grouped)
        .navigationTitle("Settings")
    }
}

#Preview {
    NavigationStack {
        SettingsView()
            .environment(AccountStore())
    }
}
