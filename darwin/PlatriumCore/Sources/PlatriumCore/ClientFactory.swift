import Apollo
import ApolloAPI
import Foundation
import PlatriumGraphQL
import PlatriumSDK

public enum ClientError: LocalizedError {
    /// The vault has no token for the account, so it has to sign in again.
    case noToken
    case invalidServerURL

    public var errorDescription: String? {
        switch self {
        case .noToken: "You're signed out. Sign in again to continue."
        case .invalidServerURL: "The server address is invalid."
        }
    }
}

/// Adds the account's bearer token to every GraphQL request. The token is read
/// from the vault each time, so signing in again in the app is picked up by the
/// File Provider extension without restarting it. A 401 means the server
/// revoked or expired it.
struct BearerInterceptor: HTTPInterceptor {
    let accountId: String
    let vault: any TokenVault
    let onUnauthorized: @Sendable (String) -> Void

    func intercept(request: URLRequest, next: NextHTTPInterceptorFunction) async throws -> HTTPResponse {
        guard let token = try vault.token(for: accountId) else {
            onUnauthorized(accountId)
            throw ClientError.noToken
        }
        var request = request
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let response = try await next(request)
        if response.response.statusCode == 401 { onUnauthorized(accountId) }
        return response
    }
}

struct AccountInterceptorProvider: InterceptorProvider {
    let accountId: String
    let vault: any TokenVault
    let onUnauthorized: @Sendable (String) -> Void

    func httpInterceptors<Operation: GraphQLOperation>(for operation: Operation) -> [any HTTPInterceptor] {
        [BearerInterceptor(accountId: accountId, vault: vault, onUnauthorized: onUnauthorized), ResponseCodeInterceptor()]
    }
}

public enum ClientFactory {
    /// A GraphQL client for one account. Each account gets its own client and
    /// cache: Apollo's store supports one endpoint only.
    public static func makeApollo(
        server: Server,
        account: Account,
        vault: any TokenVault,
        onUnauthorized: @escaping @Sendable (String) -> Void
    ) throws -> ApolloClient {
        guard let endpoint = server.graphQLURL else { throw ClientError.invalidServerURL }
        let store = ApolloStore(cache: InMemoryNormalizedCache())
        let transport = RequestChainNetworkTransport(
            urlSession: URLSession(configuration: .default),
            interceptorProvider: AccountInterceptorProvider(accountId: account.id, vault: vault, onUnauthorized: onUnauthorized),
            store: store,
            endpointURL: endpoint
        )
        return ApolloClient(networkTransport: transport, store: store)
    }

    /// A REST/transfer client carrying the account's current token. The token is
    /// baked in when the client is created, so rebuild it after signing in again.
    public static func makeSDK(server: Server, account: Account, vault: any TokenVault) throws -> PlatriumClient {
        guard let token = try vault.token(for: account.id) else { throw ClientError.noToken }
        return try PlatriumClient.withToken(baseUrl: server.restBaseURL, token: token)
    }
}
