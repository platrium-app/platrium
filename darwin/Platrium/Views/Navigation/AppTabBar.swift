import PlatriumCore
import SwiftUI

struct AppTabBar: View {
    @Environment(AccountStore.self) private var store
    @Binding var selection: SidebarSelection?
    @State private var drivesViewModel = DrivesViewModel()

    var body: some View {
        TabView {
            NavigationStack {
                HomeView()
            }
            .tabItem {
                Label("Home", systemImage: "house")
            }
            
            NavigationStack {
                List {
                    Section("Drives") {
                        DrivesMenuListView(
                            viewModel: drivesViewModel,
                            selection: $selection
                        )
                    }

                    Section("Locations") {
                        NavigationLink(value: SidebarSelection.sharedWithMe) {
                            Label("Shared with me", systemImage: "person.2")
                        }

                        NavigationLink(value: SidebarSelection.trash) {
                            Label("Trash", systemImage: "trash")
                        }
                    }
                }
                .navigationTitle("Browse")
                .toolbar {
                    ToolbarItem(placement: .primaryAction) {
                        Button {
                            store.isShowingAccountSwitcher = true
                        } label: {
                            Label(store.activeAccount?.email ?? "Accounts", systemImage: "person.crop.circle")
                        }
                    }
                }
                .navigationDestination(for: SidebarSelection.self) { sel in
                    DetailView(selection: sel)
                }
            }
            .tabItem {
                Label("Browse", systemImage: "folder")
            }

            NavigationStack {
                SettingsView()
            }
            .tabItem {
                Label("Settings", systemImage: "gear")
            }
        }
        .task(id: store.activeAccount?.id) {
            guard let apollo = store.apollo else { return }
            await drivesViewModel.load(apollo: apollo)
        }
    }
}

#Preview {
    AppTabBar(selection: .constant(.home))
        .environment(AccountStore())
}
