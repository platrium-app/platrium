#if os(macOS)
import PlatriumCore
import SwiftUI

/// The window toolbar's account control on the Mac: switch between signed-in
/// accounts straight from the menu, or open the full list.
struct AccountMenu: View {
    @Environment(AccountStore.self) private var store

    var body: some View {
        Menu {
            ForEach(store.servers) { server in
                let accounts = store.accounts(for: server)
                if !accounts.isEmpty {
                    Section(server.name) {
                        ForEach(accounts) { account in
                            Toggle(isOn: Binding(
                                get: { account.id == store.activeAccount?.id },
                                set: { _ in store.selectAccount(account) }
                            )) {
                                Text(account.email)
                            }
                        }
                    }
                }
            }
            Divider()
            Button("Manage Accounts…", systemImage: "person.2") {
                store.isShowingAccountSwitcher = true
            }
        } label: {
            Label(store.activeAccount?.email ?? "Accounts", systemImage: "person.crop.circle")
        }
        .help("Switch account")
    }
}
#endif
