import PlatriumCore
import SwiftUI

struct HomeView: View {
    @Environment(AccountStore.self) private var store

    var body: some View {
        ContentUnavailableView {
            Label("Home", systemImage: "house")
        } description: {
            if let account = store.activeAccount, let server = store.activeServer {
                Text("Signed in as \(account.email) on \(server.host).")
            }
            Text("Recent files and quick access items will be shown here.")
        }
        .navigationTitle("Home")
        #if os(iOS)
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button {
                    store.isShowingAccountSwitcher = true
                } label: {
                    Label(store.activeAccount?.email ?? "Accounts", systemImage: "person.crop.circle")
                }
            }
        }
        #endif
    }
}

#Preview {
    NavigationStack {
        HomeView()
            .environment(AccountStore())
    }
}
