import PlatriumCore
import PlatriumGraphQL
import SwiftUI
import Apollo

extension PlatriumGraphQL.GetFolderContentsQuery.Data.FolderContents.Edge.Node: Identifiable {
    var sortableSize: Swift.Int64 {
        if let sizeStr = asFile?.size, let size = Swift.Int64(sizeStr) {
            return size
        }
        return 0
    }
}

struct FolderDetailView: View {
    @Environment(AccountStore.self) private var store
    #if os(iOS)
    @Environment(\.editMode) private var editMode
    #endif
    let folderId: String

    @State private var viewModel = FolderViewModel()
    @State private var selection = Set<String>()
    @State private var sortOrder = [KeyPathComparator(\FolderContentNode.name)]
    
    private var sortedItems: [FolderContentNode] {
        viewModel.items.sorted { a, b in
            // Always keep folders at the top
            if a.type != b.type {
                return a.type == .folder
            }
            
            // Then apply selected sort order
            for comparator in sortOrder {
                let result = comparator.compare(a, b)
                if result != .orderedSame {
                    return result == .orderedAscending
                }
            }
            return false
        }
    }
    
    var body: some View {
        Group {
            if viewModel.isLoading {
                ProgressView("Loading folder contents...")
            } else if let error = viewModel.errorMessage {
                VStack(spacing: 8) {
                    Image(systemName: "exclamationmark.triangle")
                        .font(.largeTitle)
                        .foregroundStyle(.red)
                    Text("Error loading contents")
                        .font(.headline)
                    Text(error)
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                        .padding()
                    Button("Retry") {
                        Task {
                            if let apollo = store.apollo { await viewModel.loadInitial(folderId: folderId, apollo: apollo) }
                        }
                    }
                }
            } else if viewModel.items.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "folder")
                        .font(.system(size: 48))
                        .foregroundStyle(.secondary)
                    Text("This folder is empty")
                        .font(.title3)
                        .fontWeight(.semibold)
                    Text("Right-click to create a new folder or upload files.")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
            } else {
                contentView
            }
        }
        .navigationTitle(viewModel.folderName ?? "")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Menu {
                    #if os(iOS)
                    Button {
                        if editMode?.wrappedValue.isEditing == true {
                            editMode?.wrappedValue = .inactive
                        } else {
                            editMode?.wrappedValue = .active
                        }
                    } label: {
                        if editMode?.wrappedValue.isEditing == true {
                            Label("Done", systemImage: "checkmark.circle.fill")
                        } else {
                            Label("Select", systemImage: "checkmark.circle")
                        }
                    }
                    #endif
                    
                    if !selection.isEmpty {
                        Button(role: .destructive) {
                            print("Bulk delete: \(selection)")
                        } label: {
                            Label("Delete Selected (\(selection.count))", systemImage: "trash")
                        }
                    }
                    
                    Menu {
                        Button {
                            sortOrder = [KeyPathComparator(\FolderContentNode.name)]
                        } label: {
                            if sortOrder.first == KeyPathComparator(\FolderContentNode.name) {
                                Label("Name", systemImage: "checkmark")
                            } else {
                                Text("Name")
                            }
                        }
                        
                        Button {
                            sortOrder = [KeyPathComparator(\FolderContentNode.updatedAt, order: .reverse)]
                        } label: {
                            if sortOrder.first == KeyPathComparator(\FolderContentNode.updatedAt, order: .reverse) {
                                Label("Last Modified", systemImage: "checkmark")
                            } else {
                                Text("Last Modified")
                            }
                        }
                        
                        Button {
                            sortOrder = [KeyPathComparator(\FolderContentNode.sortableSize, order: .reverse)]
                        } label: {
                            if sortOrder.first == KeyPathComparator(\FolderContentNode.sortableSize, order: .reverse) {
                                Label("File Size", systemImage: "checkmark")
                            } else {
                                Text("File Size")
                            }
                        }
                    } label: {
                        Label("Sort by", systemImage: "arrow.up.arrow.down")
                    }
                    
                    Divider()
                    
                    Button(action: {
                        print("Create new folder in \(folderId)")
                    }) {
                        Label("New Folder", systemImage: "folder.badge.plus")
                    }
                } label: {
                    Image(systemName: "ellipsis")
                }
                .menuIndicator(.hidden)
            }
        }
        .task(id: [folderId, store.activeAccount?.id ?? ""]) {
            // Reload if folder changes
            if let apollo = store.apollo { await viewModel.loadInitial(folderId: folderId, apollo: apollo) }
        }
    }
    
    @ViewBuilder
    private var contentView: some View {
        #if os(macOS)
        Table(sortedItems, selection: $selection, sortOrder: $sortOrder) {
            TableColumn("Name", value: \.name) { item in
                Group {
                    if item.type == .folder {
                        NavigationLink(value: SidebarSelection.folder(id: item.id)) {
                            nameView(for: item)
                        }
                        .buttonStyle(.plain)
                    } else {
                        nameView(for: item)
                    }
                }
                .contextMenu {
                    contextMenuOptions(for: [item.id])
                }
            }
            
            TableColumn("Last Modified", value: \.updatedAt) { item in
                Text(formatDate(item.updatedAt))
                    .foregroundStyle(.secondary)
            }
            
            TableColumn("Size", value: \.sortableSize) { item in
                Text(formatSize(for: item))
                    .foregroundStyle(.secondary)
            }
        }
        .alternatingRowBackgrounds(.enabled)
        .contextMenu(forSelectionType: String.self) { selectedIds in
            contextMenuOptions(for: selectedIds)
        }
        .overlay(alignment: .bottom) {
            if viewModel.hasNextPage {
                ProgressView()
                    .controlSize(.small)
                    .padding()
                    .onAppear {
                        Task {
                            if let apollo = store.apollo { await viewModel.loadMore(folderId: folderId, apollo: apollo) }
                        }
                    }
            }
        }
        #else
        List(selection: $selection) {
            ForEach(sortedItems) { item in
                if item.type == .folder {
                    NavigationLink(value: SidebarSelection.folder(id: item.id)) {
                        iosRowView(for: item)
                    }
                    .swipeActions(edge: .trailing) {
                        Button(role: .destructive) {
                            print("Delete folder: \(item.id)")
                        } label: {
                            Label("Delete", systemImage: "trash")
                        }
                    }
                    .contextMenu {
                        contextMenuOptions(for: [item.id])
                    }
                } else {
                    Button {
                        print("Opening file: \(item.name) (ID: \(item.id))")
                    } label: {
                        iosRowView(for: item)
                    }
                    .buttonStyle(.plain)
                    .swipeActions(edge: .trailing) {
                        Button(role: .destructive) {
                            print("Delete item: \(item.id)")
                        } label: {
                            Label("Delete", systemImage: "trash")
                        }
                    }
                    .contextMenu {
                        contextMenuOptions(for: [item.id])
                    }
                }
            }
            
            if viewModel.hasNextPage {
                HStack {
                    Spacer()
                    ProgressView()
                        .onAppear {
                            Task {
                                if let apollo = store.apollo { await viewModel.loadMore(folderId: folderId, apollo: apollo) }
                            }
                        }
                    Spacer()
                }
            }
        }
        .listStyle(.inset)
        #endif
    }
    
    // MARK: - Row Views
    
    @ViewBuilder
    private func nameView(for item: FolderContentNode) -> some View {
        HStack {
            Image(systemName: fileIcon(for: item))
                .foregroundStyle(fileColor(for: item))
            Text(item.name)
                .lineLimit(1)
        }
    }
    
    @ViewBuilder
    private func iosRowView(for item: FolderContentNode) -> some View {
        HStack(spacing: 12) {
            Image(systemName: fileIcon(for: item))
                .font(.title2)
                .foregroundStyle(fileColor(for: item))
                .frame(width: 30)
            
            VStack(alignment: .leading, spacing: 4) {
                Text(item.name)
                    .font(.body)
                    .lineLimit(1)
                Text("\(formatDate(item.updatedAt)) • \(formatSize(for: item))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }
    
    // MARK: - Context Menu
    
    @ViewBuilder
    private func contextMenuOptions(for selectedIds: Set<String>) -> some View {
        if selectedIds.count == 1 {
            Button {
                // TODO: Show file information
            } label: {
                Label("Get Info", systemImage: "info.circle")
            }
            
            Button {
                // TODO: Show rename alert
            } label: {
                Label("Rename", systemImage: "pencil")
            }
        }
        
        Button(role: .destructive) {
            // TODO: Execute bulk delete
        } label: {
            Label("Delete", systemImage: "trash")
        }
    }
    
    // MARK: - Formatters
    
    private static let isoFormatterWithFractionalSeconds: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()
    
    private static let isoFormatter: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter
    }()

    private static let displayFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.dateStyle = .medium
        formatter.timeStyle = .none
        return formatter
    }()
    
    private func formatDate(_ dateString: String) -> String {
        if let date = Self.isoFormatterWithFractionalSeconds.date(from: dateString) ?? 
                      Self.isoFormatter.date(from: dateString) {
            return Self.displayFormatter.string(from: date)
        }
        return dateString
    }
    
    private func formatSize(for item: FolderContentNode) -> String {
        if item.type == .folder {
            return "--"
        }
        guard let file = item.asFile else { return "--" }
        
        let bytes = file.size
        let formatter = ByteCountFormatter()
        formatter.allowedUnits = [.useAll]
        formatter.countStyle = .file
        
        // ByteCountFormatter takes Int64, GraphQL Int64 maps to String in swift (Apollo default unless configured)
        if let bytesDouble = Double(bytes) {
            return formatter.string(fromByteCount: Swift.Int64(bytesDouble))
        }
        return bytes
    }
    
    private func fileIcon(for item: FolderContentNode) -> String {
        item.type == .folder ? "folder.fill" : MimeTypeIcon.systemImageName(for: item.asFile?.mimeType)
    }
    
    private func fileColor(for item: FolderContentNode) -> Color {
        item.type == .folder ? .blue : MimeTypeIcon.color(for: item.asFile?.mimeType)
    }
}
