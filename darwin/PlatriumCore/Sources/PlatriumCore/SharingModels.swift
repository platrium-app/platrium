import Foundation

/// A person or group that access can be given to.
public struct DirectoryPerson: Identifiable, Hashable, Sendable {
    /// USER, GROUP or TENANT.
    public var type: String
    public var personId: String
    public var name: String
    public var email: String?
    /// True when this is the signed-in user, as the server reports it.
    public var isYou: Bool

    public var id: String { "\(type):\(personId)" }
    public var isGroup: Bool { type != "USER" }

    public init(type: String, id: String, name: String, email: String? = nil, isYou: Bool = false) {
        self.type = type
        self.personId = id
        self.name = name
        self.email = email
        self.isYou = isYou
    }
}

/// One person's or group's access to an item, direct or inherited.
public struct AccessGrant: Identifiable, Hashable, Sendable {
    public var id: String
    public var subjectType: String
    public var subjectId: String
    public var subjectName: String
    public var role: String
    public var noDownload: Bool
    public var expiresAt: Date?
    public var isYou: Bool
    /// Set for inherited access: the folder or drive it comes from.
    public var inheritedFromName: String?
    /// Nil when the user can't open the source item.
    public var inheritedFromId: String?

    public var isGroup: Bool { subjectType != "USER" }
    public var isInherited: Bool { inheritedFromName != nil }

    public init(
        id: String, subjectType: String, subjectId: String, subjectName: String, role: String,
        noDownload: Bool = false, expiresAt: Date? = nil, isYou: Bool = false,
        inheritedFromName: String? = nil, inheritedFromId: String? = nil
    ) {
        self.id = id
        self.subjectType = subjectType
        self.subjectId = subjectId
        self.subjectName = subjectName
        self.role = role
        self.noDownload = noDownload
        self.expiresAt = expiresAt
        self.isYou = isYou
        self.inheritedFromName = inheritedFromName
        self.inheritedFromId = inheritedFromId
    }
}

/// Access given to everyone in a level: restricted, the organization, or anyone with the link.
public struct GeneralAccess: Hashable, Sendable {
    /// RESTRICTED, TENANT or PUBLIC.
    public var level: String
    public var role: String?
    public var noDownload: Bool
    public var expiresAt: Date?

    public init(level: String, role: String? = nil, noDownload: Bool = false, expiresAt: Date? = nil) {
        self.level = level
        self.role = role
        self.noDownload = noDownload
        self.expiresAt = expiresAt
    }
}

public struct ItemAccess: Hashable, Sendable {
    public var itemId: String
    public var owner: DirectoryPerson
    public var inheritsPermissions: Bool
    public var general: GeneralAccess
    public var grants: [AccessGrant]
    public var inherited: [AccessGrant]

    public init(
        itemId: String, owner: DirectoryPerson, inheritsPermissions: Bool, general: GeneralAccess,
        grants: [AccessGrant], inherited: [AccessGrant]
    ) {
        self.itemId = itemId
        self.owner = owner
        self.inheritsPermissions = inheritsPermissions
        self.general = general
        self.grants = grants
        self.inherited = inherited
    }
}

/// A role the server offers for an item. Labels come from the server, so new
/// roles need no app update.
public struct RoleOption: Identifiable, Hashable, Sendable {
    public var role: String
    public var label: String
    public var description: String
    /// Only viewers can be stopped from downloading.
    public var downloadOptional: Bool

    public var id: String { role }

    public init(role: String, label: String, description: String, downloadOptional: Bool) {
        self.role = role
        self.label = label
        self.description = description
        self.downloadOptional = downloadOptional
    }
}

/// A general-access level the server offers for an item.
public struct AccessLevelOption: Identifiable, Hashable, Sendable {
    public var level: String
    public var label: String
    public var blurb: String
    /// Empty for RESTRICTED.
    public var roles: [String]
    public var supportsExpiry: Bool

    public var id: String { level }
    public var takesRole: Bool { !roles.isEmpty }

    public init(level: String, label: String, blurb: String, roles: [String], supportsExpiry: Bool) {
        self.level = level
        self.label = label
        self.blurb = blurb
        self.roles = roles
        self.supportsExpiry = supportsExpiry
    }
}

/// Everything the sharing sheet needs about an item, loaded together.
public struct SharingSnapshot: Sendable {
    public var access: ItemAccess
    public var roles: [RoleOption]
    public var levels: [AccessLevelOption]

    public init(access: ItemAccess, roles: [RoleOption], levels: [AccessLevelOption]) {
        self.access = access
        self.roles = roles
        self.levels = levels
    }
}

/// A failure reported by the server, with its stable code (`FORBIDDEN`, `NOT_FOUND`, ...).
public struct SharingError: LocalizedError, Sendable {
    public var code: String?
    public var serverMessage: String?

    public init(code: String?, serverMessage: String?) {
        self.code = code
        self.serverMessage = serverMessage
    }

    /// The caller may not see or change who has access.
    public var isDenied: Bool { code == "FORBIDDEN" || code == "NOT_FOUND" }

    public var errorDescription: String? { SharingFormat.friendlyMessage(code: code, message: serverMessage) }
}
