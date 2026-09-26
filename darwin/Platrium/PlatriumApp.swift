import SwiftUI
import PlatriumSDK
import OSLog

@main
struct PlatriumApp: App {
    @State private var serverStore = ServerStore()
    @State private var isShowingServerSheet = false

    private func testSDK() async {
        guard let sdk = serverStore.sdk else { return }
        do {
            let download_session = try await sdk.files().createDownloadSession(fileId: "-WzQIsIbA5KUfDZVYMd83")
            Logger().critical("FileName: \(download_session.fileName())")
        } catch {
            print(error)
        }
    }

    var body: some Scene {
        WindowGroup {
            MainAppView()
                .environment(serverStore)
                .sheet(isPresented: $isShowingServerSheet) {
                    ServerPickerSheet()
                        .environment(serverStore)
                }
                .task {
                    await testSDK()
                }
        }
        .commands {
            CommandMenu("Server") {
                Picker("Server", selection: Binding(
                    get: { serverStore.activeServerId },
                    set: { serverStore.selectServer(id: $0) }
                )) {
                    ForEach(serverStore.servers) { server in
                        Text(server.name).tag(server.id)
                    }
                }
                .pickerStyle(.inline)

                Divider()

                Button {
                    isShowingServerSheet = true
                } label: {
                    Label("Edit Servers...", systemImage: "server.rack")
                }
            }
        }
    }
}
