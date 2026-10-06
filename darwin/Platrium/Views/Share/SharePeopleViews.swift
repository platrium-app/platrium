import PlatriumCore
import SwiftUI

/// A round avatar: initials for a person, a group glyph for a group.
struct PersonAvatar: View {
    let name: String
    let isGroup: Bool

    var body: some View {
        ZStack {
            Circle().fill(.quaternary)
            if isGroup {
                Image(systemName: "person.2.fill")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                Text(SharingFormat.initials(name))
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
            }
        }
        .frame(width: 32, height: 32)
        .accessibilityHidden(true)
    }
}

/// A person or group with a one-line detail underneath.
struct PersonRow<Trailing: View>: View {
    let name: String
    let detail: String?
    let isGroup: Bool
    @ViewBuilder var trailing: () -> Trailing

    var body: some View {
        HStack(spacing: 12) {
            PersonAvatar(name: name, isGroup: isGroup)
            VStack(alignment: .leading, spacing: 2) {
                Text(name).lineLimit(1)
                if let detail, !detail.isEmpty {
                    Text(detail)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                }
            }
            Spacer(minLength: 8)
            trailing()
        }
    }
}

extension PersonRow where Trailing == EmptyView {
    init(name: String, detail: String?, isGroup: Bool) {
        self.init(name: name, detail: detail, isGroup: isGroup) { EmptyView() }
    }
}

extension DirectoryPerson {
    /// Email for a person, "Group" for a group.
    var detailLine: String { isGroup ? "Group" : (email ?? "") }
}

extension AccessGrant {
    /// "Group · Can't download · Expires Oct 5, 2026 · From Projects", whichever apply.
    func notes(roleLabel: String? = nil) -> String {
        var parts: [String] = []
        if isGroup { parts.append("Group") }
        if noDownload { parts.append("Can't download") }
        if let expiresAt { parts.append("Expires \(expiresAt.formatted(.dateTime.month(.abbreviated).day().year()))") }
        if let inheritedFromName { parts.append("From \(inheritedFromName)") }
        return parts.joined(separator: " · ")
    }
}
