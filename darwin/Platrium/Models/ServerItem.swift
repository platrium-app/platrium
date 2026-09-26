import Foundation

struct ServerItem: Identifiable, Hashable, Codable {
    let id: String
    var name: String
    var url: String
    var isHealthy: Bool = true
}
