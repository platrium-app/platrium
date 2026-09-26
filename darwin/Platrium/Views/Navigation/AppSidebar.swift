import SwiftUI

struct AppSidebar: View {
    @Environment(ServerStore.self) private var serverStore
    @Binding var selection: SidebarSelection?
    @State private var drivesViewModel = DrivesViewModel()

    var body: some View {
        List(selection: $selection) {
            Section {
                NavigationLink(value: SidebarSelection.home) {
                    Label("Home", systemImage: "house")
                }
            }

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

            Section {
                NavigationLink(value: SidebarSelection.settings) {
                    Label("Settings", systemImage: "gear")
                }
            }
        }
        .listStyle(.sidebar)
        .navigationTitle("Platrium")
        .task(id: serverStore.activeServerId) {
            let serverId = serverStore.activeServerId
            let apollo = serverStore.apollo!
            await drivesViewModel.load(serverId: serverId, apollo: apollo)
        }
    }
}

#Preview {
    NavigationSplitView {
        AppSidebar(selection: .constant(.home))
            .environment(ServerStore())
    } detail: {
        Text("Detail View")
    }
}
