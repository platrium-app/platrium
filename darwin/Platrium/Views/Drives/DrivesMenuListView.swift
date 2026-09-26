import SwiftUI
import Apollo

typealias DriveNode = PlatriumGraphQL.GetDrivesListQuery.Data.Drife

// MARK: - ViewModel

@Observable
final class DrivesViewModel {
    private(set) var privateDrives: [DriveNode] = []
    private(set) var sharedDrives: [DriveNode] = []
    private(set) var isLoading: Bool = false
    private(set) var errorMessage: String? = nil

    func load(serverId: String, apollo: ApolloClient) async {
        guard !isLoading else { return }

        // Reset state for this server
        reset()
        isLoading = true

        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            apollo.fetch(
                query: PlatriumGraphQL.GetDrivesListQuery(),
                cachePolicy: .fetchIgnoringCacheData
            ) { [weak self] result in
                guard let self else {
                    continuation.resume()
                    return
                }
                DispatchQueue.main.async {
                    self.isLoading = false
                    switch result {
                    case .success(let graphQLResult):
                        if let drives = graphQLResult.data?.drives {
                            self.privateDrives = drives.filter {
                                $0.driveMetadata?.driveType.value == .private
                            }
                            self.sharedDrives = drives.filter {
                                $0.driveMetadata?.driveType.value == .shared
                            }
                        } else if let firstError = graphQLResult.errors?.first {
                            self.errorMessage = firstError.message
                        }
                    case .failure(let error):
                        self.errorMessage = error.localizedDescription
                    }
                    continuation.resume()
                }
            }
        }
    }

    private func reset() {
        privateDrives = []
        sharedDrives = []
        errorMessage = nil
        isLoading = false
    }
}

struct DrivesMenuListView: View {
    let viewModel: DrivesViewModel
    @Binding var selection: SidebarSelection?

    @State private var isSharedDrivesExpanded: Bool = true

    var body: some View {
        Group {
            if viewModel.isLoading {
                HStack(spacing: 8) {
                    ProgressView()
                        .controlSize(.small)
                    Text("Loading drives...")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 4)
            } else if viewModel.errorMessage != nil, viewModel.privateDrives.isEmpty, viewModel.sharedDrives.isEmpty {
                HStack(spacing: 6) {
                    Image(systemName: "exclamationmark.triangle")
                        .foregroundStyle(.red)
                    Text("Failed to load drives")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            } else {
                // Private Drives (shown directly in the list)
                ForEach(viewModel.privateDrives, id: \.id) { drive in
                    NavigationLink(value: SidebarSelection.myDrive(id: drive.id)) {
                        Label(drive.name, systemImage: "externaldrive")
                    }
                }

                // Shared Drives (collapsible, always shown)
                #if os(macOS)
                HStack {
                    NavigationLink(value: SidebarSelection.sharedDrivesOverview) {
                        Label("Shared Drives", systemImage: "externaldrive.badge.person.crop")
                    }
                    Spacer()
                    Button {
                        withAnimation {
                            isSharedDrivesExpanded.toggle()
                        }
                    } label: {
                        Image(systemName: isSharedDrivesExpanded ? "chevron.down" : "chevron.right")
                            .font(.caption2)
                            .foregroundStyle(.tertiary)
                            .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                }
                
                if isSharedDrivesExpanded {
                    ForEach(viewModel.sharedDrives, id: \.id) { drive in
                        NavigationLink(value: SidebarSelection.sharedDrive(id: drive.id)) {
                            Label(drive.name, systemImage: "externaldrive")
                        }
                        .padding(.leading, 24)
                    }
                }
                #else
                DisclosureGroup(isExpanded: $isSharedDrivesExpanded) {
                    ForEach(viewModel.sharedDrives, id: \.id) { drive in
                        NavigationLink(value: SidebarSelection.sharedDrive(id: drive.id)) {
                            Label(drive.name, systemImage: "externaldrive")
                        }
                    }
                } label: {
                    NavigationLink(value: SidebarSelection.sharedDrivesOverview) {
                        Label("Shared Drives", systemImage: "externaldrive.badge.person.crop")
                    }
                }
                #endif
            }
        }
    }
}

// MARK: - Preview

#Preview {
    let store = ServerStore()
    List {
        DrivesMenuListView(
            viewModel: DrivesViewModel(),
            selection: .constant(.home)
        )
    }
}
