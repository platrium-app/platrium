import Foundation

/// Item capabilities the server reports in `myCapabilities`.
public enum Capability {
    public static let share = "SHARE"
    public static let download = "DOWNLOAD"
    public static let edit = "EDIT"
    public static let create = "CREATE"
}

public extension Collection where Element == String {
    /// Whether a capability list (an item's `myCapabilities`) includes `capability`.
    func can(_ capability: String) -> Bool { contains(capability) }
}

/// Pure helpers for the sharing sheet.
public enum SharingFormat {
    /// "FULL_EDITOR" becomes "Full Editor", for roles the server didn't label.
    public static func humanizeRole(_ role: String) -> String {
        role.lowercased().split(separator: "_").map { $0.prefix(1).uppercased() + $0.dropFirst() }.joined(separator: " ")
    }

    /// Up to two initials for an avatar.
    public static func initials(_ name: String) -> String {
        let parts = name.split(whereSeparator: \.isWhitespace)
        switch parts.count {
        case 0: return "?"
        case 1: return String(parts[0].prefix(2)).uppercased()
        default: return (String(parts[0].prefix(1)) + String(parts[parts.count - 1].prefix(1))).uppercased()
        }
    }

    /// A drive's root has no parent. Only shared drives are shareable at the
    /// root, because a private drive's root never grants SHARE.
    public static func isDriveRoot(parentId: String?) -> Bool { parentId == nil }

    /// The page that opens an item on the server.
    public static func itemLink(serverURL: String, id: String, isFolder: Bool) -> URL? {
        URL(string: "\(serverURL)/\(isFolder ? "folder" : "file")/\(id)")
    }

    /// A message a person can act on, from the server's error code.
    public static func friendlyMessage(code: String?, message: String?) -> String {
        switch code {
        case "FORBIDDEN":
            return (message ?? "").contains("public sharing")
                ? "Your organization does not allow public sharing."
                : "You don't have permission to do that."
        case "NOT_FOUND": return "That item or person no longer exists."
        case "UNAUTHENTICATED": return "Your session has ended. Sign in again."
        case "CONFLICT": return "That already exists."
        default:
            if let message, !message.isEmpty { return message }
            return "Something went wrong. Try again."
        }
    }

    /// The last moment of the given day in the user's calendar: what "expires on this date" means.
    public static func endOfDay(_ date: Date, calendar: Calendar = .current) -> Date {
        let start = calendar.startOfDay(for: date)
        return calendar.date(bySettingHour: 23, minute: 59, second: 59, of: start) ?? date
    }

    // MARK: Dates

    private static let withFraction: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()

    private static let plain: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        return f
    }()

    /// The GraphQL `DateTime` scalar arrives as an ISO string.
    public static func parseDate(_ iso: String?) -> Date? {
        guard let iso else { return nil }
        return withFraction.date(from: iso) ?? plain.date(from: iso)
    }

    public static func isoString(_ date: Date) -> String { plain.string(from: date) }
}
