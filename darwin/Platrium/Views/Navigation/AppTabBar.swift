import SwiftUI

struct AppTabBar: View {
    @Environment(ServerStore.self) private var serverStore
    @Binding var selection: SidebarSelection?
    @State private var isShowingServerSheet = false
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
                            isShowingServerSheet = true
                        } label: {
                            Label(serverStore.activeServer.name, systemImage: "server.rack")
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
        .sheet(isPresented: $isShowingServerSheet) {
            ServerPickerSheet()
        }
        .task(id: serverStore.activeServerId) {
            let serverId = serverStore.activeServerId
            let apollo = serverStore.apollo!
            await drivesViewModel.load(serverId: serverId, apollo: apollo)
        }
    }
}

#Preview {
    AppTabBar(selection: .constant(.home))
        .environment(ServerStore())
}
