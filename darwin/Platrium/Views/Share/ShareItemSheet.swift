import PlatriumCore
import SwiftUI
#if os(macOS)
import AppKit
#else
import UIKit
#endif

/// Who can open an item: people and groups added by name, general access, and
/// whether it inherits from the folder above. The same sheet manages a shared
/// drive's members when opened on its root.
struct ShareItemSheet: View {
    @Environment(\.dismiss) private var dismiss
    @State private var viewModel: ShareViewModel
    @State private var didCopy = false

    private let linkURL: URL?

    init(target: ShareTarget, service: SharingService, serverURL: String) {
        _viewModel = State(initialValue: ShareViewModel(target: target, service: service))
        linkURL = SharingFormat.itemLink(serverURL: serverURL, id: target.id, isFolder: target.isFolder)
    }

    var body: some View {
        NavigationStack {
            Group {
                switch viewModel.phase {
                case .loading:
                    ProgressView()
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                case .denied:
                    ContentUnavailableView(
                        "Can't Manage Access",
                        systemImage: "lock",
                        description: Text("Only people who can share this \(viewModel.target.kind) can see or change who has access. Ask its owner to share it with you or to give you more access.")
                    )
                case .failed(let message):
                    ContentUnavailableView {
                        Label("Couldn't Load Sharing", systemImage: "exclamationmark.triangle")
                    } description: {
                        Text(message)
                    } actions: {
                        Button("Try Again") { Task { await viewModel.load() } }
                    }
                case .ready:
                    form
                }
            }
            .navigationTitle(viewModel.target.isDriveRoot ? "Manage Access to “\(viewModel.target.name)”" : "Share “\(viewModel.target.name)”")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
                if viewModel.phase == .ready, linkURL != nil {
                    ToolbarItem(placement: .primaryAction) {
                        Button {
                            copyLink()
                        } label: {
                            Label(didCopy ? "Copied" : "Copy Link", systemImage: didCopy ? "checkmark" : "link")
                        }
                    }
                }
            }
        }
        .task { await viewModel.load() }
        #if os(macOS)
        .frame(minWidth: 480, idealWidth: 520, minHeight: 560, idealHeight: 640)
        #else
        .presentationDetents([.large])
        #endif
    }

    // MARK: Form

    private var form: some View {
        @Bindable var viewModel = viewModel
        return Form {
            addPeopleSection
            peopleWithAccessSection
            generalAccessSection
            inheritanceSection

            if let message = viewModel.errorMessage {
                Section {
                    Label(message, systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.red)
                        .accessibilityAddTraits(.isStaticText)
                }
            }
        }
        .formStyle(.grouped)
        .disabled(viewModel.isBusy)
    }

    // MARK: Add people

    private var addPeopleSection: some View {
        @Bindable var viewModel = viewModel
        return Section("Add People") {
            ForEach(viewModel.recipients) { person in
                PersonRow(name: person.name, detail: person.detailLine, isGroup: person.isGroup) {
                    Button {
                        viewModel.remove(recipient: person)
                    } label: {
                        Image(systemName: "xmark.circle.fill").foregroundStyle(.secondary)
                    }
                    .buttonStyle(.borderless)
                    .accessibilityLabel("Remove \(person.name)")
                }
            }

            // A search field: magnifier, muted prompt, and a clear button once there is text.
            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                    .accessibilityHidden(true)
                TextField("Search", text: $viewModel.query, prompt: Text("Search for people or groups").foregroundStyle(.secondary))
                    .autocorrectionDisabled()
                    #if os(iOS)
                    .textInputAutocapitalization(.never)
                    #endif
                    .task(id: viewModel.query) { await viewModel.search() }
                    .labelsHidden()
                if !viewModel.query.isEmpty {
                    Button {
                        viewModel.query = ""
                    } label: {
                        Image(systemName: "xmark.circle.fill").foregroundStyle(.secondary)
                    }
                    .buttonStyle(.borderless)
                    .accessibilityLabel("Clear search")
                }
            }

            if viewModel.query.trimmingCharacters(in: .whitespaces).count >= 2 {
                if viewModel.results.isEmpty {
                    if viewModel.isSearching {
                        HStack(spacing: 8) { ProgressView().controlSize(.small); Text("Searching…").foregroundStyle(.secondary) }
                    } else {
                        Text("No one found").foregroundStyle(.secondary)
                    }
                } else {
                    ForEach(viewModel.results) { person in
                        Button {
                            viewModel.add(person)
                        } label: {
                            PersonRow(name: person.name, detail: person.detailLine, isGroup: person.isGroup)
                                .contentShape(Rectangle())
                        }
                        .buttonStyle(.plain)
                    }
                }
            }

            if !viewModel.recipients.isEmpty {
                Picker("Role", selection: $viewModel.newRole) {
                    ForEach(viewModel.roles) { role in Text(role.label).tag(role.role) }
                }
                Toggle("Set an end date", isOn: $viewModel.newExpiryEnabled)
                if viewModel.newExpiryEnabled {
                    DatePicker(
                        "Access ends", selection: $viewModel.newExpiry,
                        in: ShareViewModel.earliestExpiry..., displayedComponents: .date
                    )
                }
                Button("Share") { Task { await viewModel.shareWithRecipients() } }
                    .buttonStyle(.borderedProminent)
                    .frame(maxWidth: .infinity, alignment: .trailing)
            }
        }
    }

    // MARK: People with access

    @ViewBuilder
    private var peopleWithAccessSection: some View {
        if let access = viewModel.access {
            Section {
                PersonRow(
                    name: access.owner.name + (access.owner.isYou ? " (you)" : ""),
                    detail: access.owner.type == "TENANT" ? "Your organization" : access.owner.email,
                    isGroup: access.owner.isGroup
                ) {
                    Text("Owner").foregroundStyle(.secondary)
                }

                ForEach(access.grants) { grant in
                    PersonRow(
                        name: (grant.subjectName.isEmpty ? grant.subjectId : grant.subjectName) + (grant.isYou ? " (you)" : ""),
                        detail: grant.notes(),
                        isGroup: grant.isGroup
                    ) {
                        if grant.isYou {
                            Text(viewModel.roleLabel(grant.role)).foregroundStyle(.secondary)
                        } else {
                            roleMenu(for: grant)
                            Button(role: .destructive) {
                                Task { await viewModel.revoke(grant) }
                            } label: {
                                Image(systemName: "trash")
                            }
                            .buttonStyle(.borderless)
                            .accessibilityLabel("Remove access for \(grant.subjectName)")
                        }
                    }
                }

                ForEach(access.inherited) { grant in
                    PersonRow(
                        name: grant.subjectName.isEmpty ? grant.subjectId : grant.subjectName,
                        detail: grant.notes(),
                        isGroup: grant.isGroup
                    ) {
                        Text(viewModel.roleLabel(grant.role)).foregroundStyle(.secondary)
                    }
                }
            } header: {
                Text("People with Access")
            } footer: {
                if access.grants.isEmpty && access.inherited.isEmpty {
                    Text("Only the owner can open this. Add people above to share it.")
                }
            }
        }
    }

    private func roleMenu(for grant: AccessGrant) -> some View {
        Picker("Role", selection: Binding(
            get: { grant.role },
            set: { newRole in Task { await viewModel.changeRole(of: grant, to: newRole) } }
        )) {
            ForEach(viewModel.roles) { role in Text(role.label).tag(role.role) }
            // A role this app doesn't know is still shown, not silently replaced.
            if !viewModel.roles.contains(where: { $0.role == grant.role }) {
                Text(viewModel.roleLabel(grant.role)).tag(grant.role)
            }
        }
        .labelsHidden()
        .pickerStyle(.menu)
        .fixedSize()
    }

    // MARK: General access

    @ViewBuilder
    private var generalAccessSection: some View {
        if let general = viewModel.general, let level = viewModel.currentLevel {
            let role = viewModel.roleOption(general.role)
            Section {
                Picker("Who can open this", selection: Binding(
                    get: { general.level },
                    set: { newLevel in
                        if let option = viewModel.levels.first(where: { $0.level == newLevel }) {
                            Task { await viewModel.setLevel(option) }
                        }
                    }
                )) {
                    ForEach(levelChoices(current: level)) { option in
                        Label(option.label, systemImage: Self.icon(for: option.level)).tag(option.level)
                    }
                }

                if level.takesRole {
                    if level.roles.count > 1 {
                        Picker("Role", selection: Binding(
                            get: { general.role ?? level.roles[0] },
                            set: { newRole in Task { await viewModel.setGeneralRole(newRole) } }
                        )) {
                            ForEach(level.roles, id: \.self) { Text(viewModel.roleLabel($0)).tag($0) }
                        }
                    } else if let only = level.roles.first {
                        LabeledContent("Role", value: viewModel.roleLabel(only))
                    }

                    if role?.downloadOptional == true {
                        Toggle("Allow downloading", isOn: Binding(
                            get: { !general.noDownload },
                            set: { allowed in Task { await viewModel.setGeneralDownloads(allowed: allowed) } }
                        ))
                    }

                    if level.supportsExpiry {
                        Toggle("Set an end date", isOn: Binding(
                            get: { general.expiresAt != nil },
                            set: { on in Task { await viewModel.setGeneralExpiry(on ? ShareViewModel.defaultExpiry : nil) } }
                        ))
                        if let expiresAt = general.expiresAt {
                            DatePicker(
                                "Access ends",
                                selection: Binding(
                                    get: { expiresAt },
                                    set: { date in Task { await viewModel.setGeneralExpiry(date) } }
                                ),
                                in: ShareViewModel.earliestExpiry..., displayedComponents: .date
                            )
                        }
                    }
                }
            } header: {
                Text("General Access")
            } footer: {
                if !level.blurb.isEmpty { Text(level.blurb) }
            }
        }
    }

    /// The server's offered levels, plus the current one if it no longer offers it.
    private func levelChoices(current: AccessLevelOption) -> [AccessLevelOption] {
        viewModel.levels.contains { $0.level == current.level } ? viewModel.levels : viewModel.levels + [current]
    }

    private static func icon(for level: String) -> String {
        switch level {
        case "RESTRICTED": "lock"
        case "TENANT": "building.2"
        case "PUBLIC": "globe"
        default: "person.2"
        }
    }

    // MARK: Inheritance

    @ViewBuilder
    private var inheritanceSection: some View {
        if let access = viewModel.access, !viewModel.target.isDriveRoot {
            let kind = viewModel.target.kind
            Section {
                Button(access.inheritsPermissions ? "Restrict Access" : "Inherit Access") {
                    Task { await viewModel.setInheritance(!access.inheritsPermissions) }
                }
            } header: {
                Text("Inherited Access")
            } footer: {
                Text(access.inheritsPermissions
                     ? "People with access to the folder above can also open this \(kind)."
                     : "Only the people listed here can open this \(kind). Access from the folder above doesn't apply.")
            }
        }
    }

    // MARK: Copy link

    private func copyLink() {
        guard let linkURL else { return }
        #if os(macOS)
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(linkURL.absoluteString, forType: .string)
        #else
        UIPasteboard.general.url = linkURL
        #endif
        didCopy = true
        Task {
            try? await Task.sleep(for: .seconds(2))
            didCopy = false
        }
    }
}
