import PlatriumCore
import SwiftUI

struct MainAppView: View {
    @Environment(AccountStore.self) private var store
    @State private var selection: SidebarSelection? = .home
    @State private var path = NavigationPath()

    var body: some View {
        @Bindable var store = store

        Group {
            if let storageError = store.storageError {
                ContentUnavailableView(
                    "Can't Open Your Accounts",
                    systemImage: "externaldrive.badge.exclamationmark",
                    description: Text(storageError)
                )
            } else if store.accounts.isEmpty {
                // First run, or a server was added but nobody signed in yet.
                ServerSetupView()
            } else if let account = store.activeAccount, let server = store.activeServer, account.status == .needsReauth {
                SignedOutView(account: account, server: server)
            } else {
                content
            }
        }
        .sheet(isPresented: $store.isShowingAccountSwitcher) {
            AccountSwitcherSheet()
        }
        // Another account's drive and folder ids mean nothing here.
        .onChange(of: store.activeAccount?.id) { _, _ in
            selection = .home
            path.removeLast(path.count)
        }
    }

    @ViewBuilder
    private var content: some View {
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
        #if os(macOS)
        .toolbar {
            ToolbarItem(placement: .primaryAction) { AccountMenu() }
        }
        #endif
        .onChange(of: selection) { _, _ in
            path.removeLast(path.count)
        }
    }
}

#Preview {
    MainAppView()
        .environment(AccountStore())
}
