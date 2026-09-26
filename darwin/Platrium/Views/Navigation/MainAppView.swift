import SwiftUI

struct MainAppView: View {
    @State private var selection: SidebarSelection? = .home
    @State private var path = NavigationPath()

    var body: some View {
        #if os(iOS)
        if UIDevice.current.userInterfaceIdiom == .phone {
            AppTabBar(selection: $selection)
        } else {
            macOSOrPadSplitView
        }
        #else
        macOSOrPadSplitView
        #endif
    }

    private var macOSOrPadSplitView: some View {
        NavigationSplitView {
            AppSidebar(selection: $selection)
                .navigationSplitViewColumnWidth(min: 180, ideal: 200, max: 280)
        } detail: {
            NavigationStack(path: $path) {
                if let selection {
                    DetailView(selection: selection)
                        .navigationDestination(for: SidebarSelection.self) { sel in
                            DetailView(selection: sel)
                        }
                } else {
                    ContentUnavailableView(
                        "Select a Drive",
                        systemImage: "sidebar.left",
                        description: Text("Choose an item from the sidebar to view details.")
                    )
                }
            }
        }
        .onChange(of: selection) { _, _ in
            path.removeLast(path.count)
        }
    }
}

#Preview {
    MainAppView()
}
