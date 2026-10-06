import Foundation
import PlatriumCore

/// The item the sharing sheet is about.
struct ShareTarget: Identifiable, Hashable {
    let id: String
    let name: String
    let isFolder: Bool
    /// Nil for a drive's root.
    let parentId: String?

    /// Only shared drives are shareable at the root, and their members hold more roles.
    var isDriveRoot: Bool { SharingFormat.isDriveRoot(parentId: parentId) }
    var kind: String { isFolder ? "folder" : "file" }

    var sheetTitle: String { isDriveRoot ? "Manage Access" : "Share" }
}
