import PlatriumCore
import SwiftUI

/// Every server and the accounts signed in on it. Pick one to switch to it.
struct AccountSwitcherSheet: View {
    @Environment(AccountStore.self) private var store
    @Environment(\.dismiss) private var dismiss

    @State private var setup: SetupRequest?

    /// What the "add" sheet should start with.
    private struct SetupRequest: Identifiable {
        let id = UUID()
        let server: Server?
    }

    var body: some View {
        NavigationStack {
            List {
                ForEach(store.servers) { server in
                    Section {
                        ForEach(store.accounts(for: server)) { account in
                            Button {
                                store.selectAccount(account)
                                dismiss()
                            } label: {
                                accountRow(account)
                            }
                            .buttonStyle(.plain)
                        }

                        Button {
                            setup = SetupRequest(server: server)
                        } label: {
                            Label("Add Account", systemImage: "person.badge.plus")
                        }
                    } header: {
                        Text(server.name)
                    } footer: {
                        Text(server.url)
                    }
                }
            }
            .navigationTitle("Accounts")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Done") { dismiss() }
                }
                ToolbarItem(placement: .primaryAction) {
                    Button {
                        setup = SetupRequest(server: nil)
                    } label: {
                        Label("Add Server", systemImage: "plus")
                    }
                }
            }
        }
        .sheet(item: $setup) { request in
            ServerSetupView(server: request.server, isModal: true)
        }
        #if os(macOS)
        .frame(minWidth: 420, minHeight: 380)
        #else
        .presentationDetents([.medium, .large])
        #endif
    }

    private func accountRow(_ account: Account) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(account.email)
                if account.status == .needsReauth {
                    Text("Sign in again")
                        .font(.caption)
                        .foregroundStyle(.red)
                }
            }
            Spacer()
            if account.id == store.activeAccount?.id {
                Image(systemName: "checkmark")
                    .foregroundStyle(.tint)
                    .accessibilityLabel("Current account")
            }
        }
        .contentShape(Rectangle())
    }
}
