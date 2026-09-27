import SwiftUI
import FileProvider

struct MainAppView: View {
    @State private var selection: SidebarSelection? = .home
    @State private var path = NavigationPath()

    var body: some View {
        Group {
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
        .onAppear {
            registerTestDrive()
        }
    }
    private func registerTestDrive() {
        Task {
            do {
                // 1. Fetch and remove all existing domains to clear the macOS cache
                let existingDomains = try await NSFileProviderManager.domains()
                for domain in existingDomains {
                    try await NSFileProviderManager.remove(domain)
                }
                
                // 2. Create a fresh domain with a random UUID to guarantee a clean slate
                let randomSuffix = UUID().uuidString.prefix(4)
                let newDomain = NSFileProviderDomain(
                    identifier: NSFileProviderDomainIdentifier(rawValue: "platrium_dev_\(randomSuffix)"),
                    displayName: "Platrium Dev \(randomSuffix)"
                )
                
                try await NSFileProviderManager.add(newDomain)
                print("Successfully nuked old drives and added fresh domain: '\(newDomain.displayName)'!")
            } catch {
                print("Failed to register test drive: \(error.localizedDescription)")
            }
        }
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
