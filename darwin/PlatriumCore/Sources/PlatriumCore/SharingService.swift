import Apollo
import ApolloAPI
import Foundation
import PlatriumGraphQL

/// Reads and changes who has access to an item. Everything is decided by the
/// server; this only carries requests and maps the answers into plain values.
public struct SharingService: Sendable {
    private let apollo: ApolloClient

    public init(apollo: ApolloClient) {
        self.apollo = apollo
    }

    // MARK: Reading

    /// Loads access, offered roles and offered general-access levels together.
    /// Throws a `SharingError` whose `isDenied` is true when the user may not manage access.
    public func load(itemId: String) async throws -> SharingSnapshot {
        async let access = access(itemId: itemId)
        async let roles = roles(itemId: itemId)
        async let levels = levels(itemId: itemId)
        return try await SharingSnapshot(access: access, roles: roles, levels: levels)
    }

    public func access(itemId: String) async throws -> ItemAccess {
        let response = try await apollo.fetch(query: ItemAccessQuery(itemId: itemId), cachePolicy: .networkOnly)
        let data = try Self.data(from: response)
        let a = data.itemAccess
        return ItemAccess(
            itemId: a.itemId,
            owner: DirectoryPerson(type: a.owner.type, id: a.owner.id, name: a.owner.name, email: a.owner.email, isYou: a.owner.isYou),
            inheritsPermissions: a.inheritsPermissions,
            general: GeneralAccess(
                level: a.generalAccess.level, role: a.generalAccess.role,
                noDownload: a.generalAccess.noDownload, expiresAt: SharingFormat.parseDate(a.generalAccess.expiresAt)
            ),
            grants: a.grants.map {
                AccessGrant(
                    id: $0.id, subjectType: $0.subjectType, subjectId: $0.subjectId, subjectName: $0.subjectName,
                    role: $0.role, noDownload: $0.noDownload, expiresAt: SharingFormat.parseDate($0.expiresAt), isYou: $0.isYou
                )
            },
            inherited: a.inherited.map {
                AccessGrant(
                    id: $0.id, subjectType: $0.subjectType, subjectId: $0.subjectId, subjectName: $0.subjectName,
                    role: $0.role, noDownload: $0.noDownload, expiresAt: SharingFormat.parseDate($0.expiresAt), isYou: $0.isYou,
                    inheritedFromName: $0.inheritedFrom?.name ?? "above", inheritedFromId: $0.inheritedFrom?.id
                )
            }
        )
    }

    public func roles(itemId: String) async throws -> [RoleOption] {
        let response = try await apollo.fetch(query: ShareRolesQuery(itemId: itemId), cachePolicy: .networkOnly)
        return try Self.data(from: response).shareRoles.map {
            RoleOption(role: $0.role, label: $0.label, description: $0.description, downloadOptional: $0.downloadOptional)
        }
    }

    public func levels(itemId: String) async throws -> [AccessLevelOption] {
        let response = try await apollo.fetch(query: GeneralAccessOptionsQuery(itemId: itemId), cachePolicy: .networkOnly)
        return try Self.data(from: response).generalAccessOptions.map {
            AccessLevelOption(level: $0.level, label: $0.label, blurb: $0.blurb, roles: $0.roles, supportsExpiry: $0.supportsExpiry)
        }
    }

    /// People and groups matching `query`, leaving out anyone who already has access to `itemId`.
    public func search(_ query: String, excludingAccessTo itemId: String) async throws -> [DirectoryPerson] {
        let response = try await apollo.fetch(
            query: SearchDirectoryQuery(query: query, first: .some(8), exclude: .some(itemId)),
            cachePolicy: .networkOnly
        )
        return try Self.data(from: response).searchDirectory.map {
            DirectoryPerson(type: $0.type, id: $0.id, name: $0.name, email: $0.email)
        }
    }

    // MARK: Changing

    /// Gives someone access, or changes their role: the server treats a repeat for
    /// the same subject as an update.
    public func share(
        itemId: String, subjectType: String, subjectId: String, role: String,
        noDownload: Bool = false, expiresAt: Date? = nil
    ) async throws {
        let input = ShareInput(
            itemId: itemId, subjectType: subjectType, subjectId: subjectId, role: role,
            noDownload: .some(noDownload),
            expiresAt: expiresAt.map { .some(SharingFormat.isoString($0)) } ?? .none
        )
        _ = try Self.data(from: try await apollo.perform(mutation: ShareItemMutation(input: input)))
    }

    public func revoke(grantId: String) async throws {
        _ = try Self.data(from: try await apollo.perform(mutation: RevokeAccessMutation(grantId: grantId)))
    }

    /// Sets who can open the item without being named. A level with no roles
    /// (RESTRICTED) takes no role.
    public func setGeneralAccess(
        itemId: String, level: String, role: String?, noDownload: Bool, expiresAt: Date?
    ) async throws {
        let input = GeneralAccessInput(
            itemId: itemId, level: level,
            role: role.map { .some($0) } ?? .none,
            noDownload: .some(noDownload),
            expiresAt: expiresAt.map { .some(SharingFormat.isoString($0)) } ?? .none
        )
        _ = try Self.data(from: try await apollo.perform(mutation: SetGeneralAccessMutation(input: input)))
    }

    public func setInheritance(itemId: String, inherit: Bool) async throws {
        _ = try Self.data(from: try await apollo.perform(mutation: SetInheritanceMutation(itemId: itemId, inherit: inherit)))
    }

    // MARK: Errors

    /// GraphQL reports failures in the response body with HTTP 200, so a missing
    /// `data` means the first error says what happened.
    private static func data<Operation: GraphQLOperation>(from response: GraphQLResponse<Operation>) throws -> Operation.Data {
        if let error = response.errors?.first {
            throw SharingError(code: error.extensions?["code"] as? String, serverMessage: error.message)
        }
        guard let data = response.data else {
            throw SharingError(code: nil, serverMessage: nil)
        }
        return data
    }
}
