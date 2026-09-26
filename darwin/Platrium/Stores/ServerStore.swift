import SwiftUI
import Apollo
import PlatriumSDK
import OSLog

@Observable
final class ServerStore {
    var servers: [ServerItem] = [
        ServerItem(id: "server-1", name: "Staging Server", url: "http://172.20.0.179:3000"),
    ]
    
    var activeServerId: String = "server-1" {
        didSet {
            rebuildClients()
        }
    }
    
    var activeServer: ServerItem {
        servers.first(where: { $0.id == activeServerId }) ?? servers[0]
    }

    private(set) var apollo: ApolloClient!
    private(set) var sdk: PlatriumClient?

    init() {
        rebuildClients()
    }
    
    func selectServer(id: String) {
        activeServerId = id
    }
    
    func addServer(name: String, url: String) {
        let cleanUrl = url.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        let newServer = ServerItem(id: UUID().uuidString, name: name, url: cleanUrl)
        servers.append(newServer)
        activeServerId = newServer.id
    }

    func deleteServer(at offsets: IndexSet) {
        servers.remove(atOffsets: offsets)
        if !servers.contains(where: { $0.id == activeServerId }), let first = servers.first {
            activeServerId = first.id
        }
    }

    private func rebuildClients() {
        let baseUrl = activeServer.url.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        
        // 1. Rebuild Apollo GraphQL Client (/graphql)
        if let gqlUrl = URL(string: "\(baseUrl)/graphql") {
            self.apollo = ApolloClient(url: gqlUrl)
        }
        
        // 2. Rebuild Platrium SDK REST Client (/api)
        do {
            self.sdk = try PlatriumClient(baseUrl: "\(baseUrl)/api")
        } catch {
            Logger().error("Failed to initialize PlatriumClient SDK: \(error.localizedDescription)")
            self.sdk = nil
        }
    }
}
