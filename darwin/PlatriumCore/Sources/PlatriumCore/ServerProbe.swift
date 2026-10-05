import Apollo
import Foundation
import PlatriumGraphQL

public struct ServerDetails: Equatable, Sendable {
    public var version: String
    public var edition: String
    public var isMultiTenant: Bool
}

public enum ServerProbeError: LocalizedError {
    case unreachable(String)
    case notAPlatriumServer

    public var errorDescription: String? {
        switch self {
        case .unreachable(let reason): "Couldn't reach the server: \(reason)"
        case .notAPlatriumServer: "That address doesn't look like a Platrium server."
        }
    }
}

public enum ServerProbe {
    /// Asks a server who it is, without credentials, through the typed
    /// `serverInfo` query that the web login page also uses before sign-in.
    public static func fetch(serverURL: String) async throws -> ServerDetails {
        guard let endpoint = URL(string: serverURL + "/graphql") else { throw ServerProbeError.notAPlatriumServer }
        let client = ApolloClient(url: endpoint)

        let response: GraphQLResponse<PlatriumGraphQL.GetServerInfoQuery>
        do {
            response = try await client.fetch(query: PlatriumGraphQL.GetServerInfoQuery(), cachePolicy: .networkOnly)
        } catch let error as URLError {
            throw ServerProbeError.unreachable(error.localizedDescription)
        } catch {
            // The host answered but not with our GraphQL schema.
            throw ServerProbeError.notAPlatriumServer
        }
        guard let info = response.data?.serverInfo else { throw ServerProbeError.notAPlatriumServer }
        return ServerDetails(version: info.version, edition: info.edition, isMultiTenant: info.isMultiTenant)
    }
}
