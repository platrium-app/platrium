import Foundation

public enum ServerURLError: LocalizedError, Equatable {
    case empty
    case invalid
    case unsupportedScheme(String)

    public var errorDescription: String? {
        switch self {
        case .empty: "Enter your server's address."
        case .invalid: "That doesn't look like a valid server address."
        case .unsupportedScheme(let s): "Only http and https servers are supported, not “\(s)”."
        }
    }
}

public enum ServerURL {
    /// Turns whatever the user typed into the canonical server URL: lowercase
    /// scheme and host, an optional port and path prefix, no query, fragment or
    /// trailing slash.
    ///
    /// A missing scheme defaults to `https`, except for addresses that are
    /// obviously on the local network (`localhost`, `*.local`, IP literals,
    /// single-label hosts), which default to `http` for development servers.
    public static func normalize(_ input: String) throws -> String {
        let trimmed = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { throw ServerURLError.empty }

        let withScheme: String
        if trimmed.contains("://") {
            withScheme = trimmed
        } else {
            let host = trimmed.split(separator: "/").first.map(String.init) ?? trimmed
            withScheme = (isLocalHost(host) ? "http://" : "https://") + trimmed
        }

        guard var components = URLComponents(string: withScheme),
              let scheme = components.scheme?.lowercased(),
              let host = components.host?.lowercased(), !host.isEmpty
        else { throw ServerURLError.invalid }
        guard scheme == "http" || scheme == "https" else { throw ServerURLError.unsupportedScheme(scheme) }
        guard components.user == nil, components.password == nil else { throw ServerURLError.invalid }

        components.scheme = scheme
        components.host = host
        components.query = nil
        components.fragment = nil
        while components.path.hasSuffix("/") { components.path.removeLast() }

        guard let url = components.string else { throw ServerURLError.invalid }
        return url
    }

    /// Whether a host (optionally with a port) is on the local network.
    static func isLocalHost(_ hostAndPort: String) -> Bool {
        let host = hostAndPort.split(separator: ":", maxSplits: 1).first.map(String.init)?.lowercased() ?? hostAndPort.lowercased()
        if host == "localhost" || host.hasSuffix(".local") || !host.contains(".") { return true }
        let parts = host.split(separator: ".")
        return parts.count == 4 && parts.allSatisfy { UInt8($0) != nil }
    }
}
