import PlatriumCore
import SwiftUI

@main
struct PlatriumApp: App {
    @State private var store = AccountStore()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            MainAppView()
                .environment(store)
                .task { await store.start() }
        }
        // The File Provider extension may have marked an account as signed out meanwhile.
        .onChange(of: scenePhase) { _, phase in
            if phase == .active { store.reload() }
        }
        .commands {
            CommandMenu("Accounts") {
                ForEach(store.servers) { server in
                    let accounts = store.accounts(for: server)
                    if !accounts.isEmpty {
                        Section(server.name) {
                            ForEach(accounts) { account in
                                Toggle(account.email, isOn: Binding(
                                    get: { account.id == store.activeAccount?.id },
                                    set: { _ in store.selectAccount(account) }
                                ))
                            }
                        }
                    }
                }

                Divider()

                Button {
                    store.isShowingAccountSwitcher = true
                } label: {
                    Label("Manage Accounts…", systemImage: "person.2")
                }
            }
        }
    }
}
