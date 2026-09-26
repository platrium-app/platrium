import SwiftUI

struct HomeView: View {
    @Environment(ServerStore.self) private var serverStore
    @State private var isShowingServerSheet = false

    var body: some View {
        VStack(spacing: 16) {
            Image(systemName: "house")
                .font(.system(size: 48))
                .foregroundStyle(.tint)
            Text("Home")
                .font(.title)
                .bold()
            Text("Connected to \(serverStore.activeServer.name) (\(serverStore.activeServer.url))")
                .font(.subheadline)
                .foregroundStyle(.secondary)
            Text("Recent files and quick access items will be shown here.")
                .foregroundStyle(.secondary)
        }
        .padding()
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .navigationTitle("Home")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button {
                    isShowingServerSheet = true
                } label: {
                    Label(serverStore.activeServer.name, systemImage: "server.rack")
                }
            }
        }
        .sheet(isPresented: $isShowingServerSheet) {
            ServerPickerSheet()
        }
    }
}

#Preview {
    NavigationStack {
        HomeView()
            .environment(ServerStore())
    }
}
