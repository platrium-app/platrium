import SwiftUI

struct ServerPickerSheet: View {
    @Environment(ServerStore.self) private var serverStore
    @Environment(\.dismiss) private var dismiss
    @State private var isShowingAddServer = false

    var body: some View {
        #if os(iOS)
        iOSServerPickerView(isShowingAddServer: $isShowingAddServer)
            .sheet(isPresented: $isShowingAddServer) {
                AddServerSheet()
            }
        #else
        macOSServerPickerView(isShowingAddServer: $isShowingAddServer)
            .sheet(isPresented: $isShowingAddServer) {
                AddServerSheet()
            }
        #endif
    }
}

#if os(iOS)
struct iOSServerPickerView: View {
    @Environment(ServerStore.self) private var serverStore
    @Environment(\.dismiss) private var dismiss
    @Binding var isShowingAddServer: Bool

    var body: some View {
        NavigationStack {
            List {
                Section {
                    ForEach(serverStore.servers) { server in
                        Button {
                            serverStore.selectServer(id: server.id)
                            dismiss()
                        } label: {
                            HStack {
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(server.name)
                                        .font(.body)
                                        .bold()
                                        .foregroundStyle(.primary)
                                    Text(server.url)
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                Spacer()
                                if serverStore.activeServerId == server.id {
                                    Image(systemName: "checkmark.circle.fill")
                                        .font(.title3)
                                        .foregroundStyle(.blue)
                                }
                            }
                        }
                    }
                    .onDelete(perform: serverStore.deleteServer)
                } header: {
                    Text("Servers")
                } footer: {
                    Text("Select a server to switch your active connection, or swipe left on a server to remove it.")
                }
            }
            .listStyle(.insetGrouped)
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button {
                        isShowingAddServer = true
                    } label: {
                        Image(systemName: "plus")
                            .font(.system(size: 16, weight: .bold))
                            .foregroundStyle(.primary)
                            .frame(width: 36, height: 36)
                            .background(.ultraThinMaterial, in: Circle())
                    }
                    .buttonStyle(.plain)
                }
            }
        }
        .presentationDetents([.medium, .large])
    }
}
#endif

#if os(macOS)
struct macOSServerPickerView: View {
    @Environment(ServerStore.self) private var serverStore
    @Environment(\.dismiss) private var dismiss
    @Binding var isShowingAddServer: Bool

    var body: some View {
        VStack(spacing: 0) {
            // Header Bar
            HStack {
                Image(systemName: "server.rack")
                    .font(.title3)
                    .foregroundStyle(.tint)
                
                Text("Servers")
                    .font(.headline)
                    .bold()

                Spacer()

                Button {
                    isShowingAddServer = true
                } label: {
                    Image(systemName: "plus")
                        .font(.system(size: 14, weight: .bold))
                        .foregroundStyle(.primary)
                        .frame(width: 30, height: 30)
                        .background(.ultraThinMaterial, in: Circle())
                }
                .buttonStyle(.plain)
            }
            .padding(.horizontal, 16)
            .padding(.top, 16)
            .padding(.bottom, 10)

            List {
                ForEach(serverStore.servers) { server in
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(server.name)
                                .font(.body)
                                .bold()
                                .foregroundStyle(.primary)
                            Text(server.url)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        
                        Spacer()
                        
                        if serverStore.activeServerId == server.id {
                            Image(systemName: "checkmark.circle.fill")
                                .font(.title3)
                                .foregroundStyle(.blue)
                        }
                    }
                    .padding(.vertical, 2)
                    .contentShape(Rectangle())
                    .onTapGesture {
                        serverStore.selectServer(id: server.id)
                        dismiss()
                    }
                    .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                        Button(role: .destructive) {
                            if let index = serverStore.servers.firstIndex(where: { $0.id == server.id }) {
                                serverStore.deleteServer(at: IndexSet(integer: index))
                            }
                        } label: {
                            Label("Delete", systemImage: "trash")
                        }
                    }
                }
                .onDelete(perform: serverStore.deleteServer)
            }
            .listStyle(.sidebar)

            HStack {
                Spacer()
                Button("Done") {
                    dismiss()
                }
                .keyboardShortcut(.defaultAction)
            }
            .padding(16)
        }
        .frame(width: 380, height: 280)
    }
}
#endif

struct AddServerSheet: View {
    @Environment(ServerStore.self) private var serverStore
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var url = "http://"

    var body: some View {
        NavigationStack {
            Form {
                Section("Server Details") {
                    TextField("Server Name", text: $name)
                    TextField("Server URL", text: $url)
                }
            }
            .navigationTitle("Add Server")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #else
            .frame(width: 340, height: 180)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Add") {
                        if !name.isEmpty && !url.isEmpty {
                            serverStore.addServer(name: name, url: url)
                        }
                        dismiss()
                    }
                    .disabled(name.isEmpty || url.isEmpty)
                }
            }
        }
    }
}

#Preview {
    ServerPickerSheet()
        .environment(ServerStore())
}
