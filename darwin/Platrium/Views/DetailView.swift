import SwiftUI

struct DetailView: View {
    let selection: SidebarSelection

    var body: some View {
        VStack(spacing: 16) {
            switch selection {
            case .home:
                Label("Home", systemImage: "house")
                    .font(.title)
            case .myDrive(let id):
                FolderDetailView(folderId: id)
            case .sharedDrivesOverview:
                Label("Shared Drives Overview", systemImage: "externaldrive.badge.person.crop")
                    .font(.title)
            case .sharedDrive(let id):
                FolderDetailView(folderId: id)
            case .sharedWithMe:
                Label("Shared with me", systemImage: "person.2")
                    .font(.title)
            case .trash:
                Label("Trash", systemImage: "trash")
                    .font(.title)
            case .settings:
                Label("Settings", systemImage: "gear")
                    .font(.title)
            case .folder(let id):
                FolderDetailView(folderId: id)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

#Preview {
    DetailView(selection: .home)
}
