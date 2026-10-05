import PlatriumCore
import SwiftUI

/// Connect to a server, then sign in. Shown on first launch, and from the
/// account switcher to add a server (or, with `server`, another account on one).
struct ServerSetupView: View {
    private enum Step: Equatable {
        case address
        case signIn(Server, ServerDetails?)
    }

    @Environment(AccountStore.self) private var store
    @Environment(\.webAuthenticationSession) private var webAuthenticationSession
    @Environment(\.dismiss) private var dismiss

    private let isModal: Bool
    @State private var step: Step
    @State private var address = ""
    @State private var isWorking = false
    @State private var errorMessage: String?
    @FocusState private var addressFocused: Bool

    init(server: Server? = nil, isModal: Bool = false) {
        self.isModal = isModal
        // With no explicit server, resume at sign-in if one was added but never signed in to.
        _step = State(initialValue: server.map { .signIn($0, nil) } ?? .address)
    }

    var body: some View {
        NavigationStack {
            Group {
                #if os(macOS)
                macContent
                #else
                iosContent
                #endif
            }
            #if os(iOS)
            .navigationTitle(isModal ? "Add Account" : "")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                if isModal {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { dismiss() }
                    }
                }
            }
            #endif
            .disabled(isWorking)
        }
        #if os(macOS)
        // As a sheet the content has no window to size it.
        .frame(minWidth: isModal ? 480 : 0, minHeight: isModal ? 440 : 0)
        #endif
        .onAppear {
            // Land on sign-in when a server is already stored but nobody has signed in yet.
            if case .address = step, !isModal, let server = store.activeServer {
                step = .signIn(server, nil)
            }
        }
    }

    // MARK: iOS

    #if os(iOS)
    private var iosContent: some View {
        Form {
            Section {
                heroStack
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 8)
            }
            .listRowBackground(Color.clear)

            switch step {
            case .address:
                Section {
                    addressField
                } footer: {
                    Text("Ask your administrator if you're not sure.")
                }
            case .signIn(let server, let details):
                Section {
                    LabeledContent("Address", value: server.host)
                    if let details { LabeledContent("Version", value: details.version) }
                } header: {
                    Text("Server")
                } footer: {
                    Text("You'll sign in on your server's web page. Your password is never shared with this app.")
                }
            }

            if let errorMessage {
                Section {
                    Label(errorMessage, systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.red)
                }
            }
        }
        .formStyle(.grouped)
        // The actions sit outside the Form so they aren't wrapped in a grouped-section card.
        .safeAreaInset(edge: .bottom, spacing: 0) {
            VStack(spacing: 20) {
                primaryButton
                    .frame(maxWidth: .infinity)
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                if case .signIn = step { differentServerButton.buttonStyle(.borderless) }
            }
            .padding(.horizontal, 20)
            .padding(.top, 8)
            .padding(.bottom, 20)
        }
    }
    #endif

    // MARK: macOS

    #if os(macOS)
    /// A centered column with a natural-width default button, the way Mac sign-in
    /// windows are laid out, instead of the full-width iOS button.
    private var macContent: some View {
        VStack(spacing: 22) {
            heroStack

            VStack(alignment: .leading, spacing: 8) {
                switch step {
                case .address:
                    addressField
                        .textFieldStyle(.roundedBorder)
                        .controlSize(.large)
                    Text("Ask your administrator if you're not sure.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                case .signIn(let server, let details):
                    GroupBox("Server") {
                        VStack(spacing: 8) {
                            LabeledContent("Address", value: server.host)
                            if let details {
                                Divider()
                                LabeledContent("Version", value: details.version)
                            }
                        }
                        .padding(.vertical, 4)
                        .frame(maxWidth: .infinity)
                    }
                    .frame(maxWidth: .infinity)
                    Text("You'll sign in on your server's web page. Your password is never shared with this app.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }

                if let errorMessage {
                    Label(errorMessage, systemImage: "exclamationmark.triangle")
                        .font(.callout)
                        .foregroundStyle(.red)
                        .fixedSize(horizontal: false, vertical: true)
                        .padding(.top, 4)
                }
            }

            HStack {
                if isModal {
                    Button("Cancel") { dismiss() }
                        .keyboardShortcut(.cancelAction)
                }
                if case .signIn = step {
                    differentServerButton
                }
                Spacer()
                primaryButton
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
            }
        }
        .frame(maxWidth: 420)
        .padding(32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    #endif

    // MARK: Shared pieces

    private var heroStack: some View {
        VStack(spacing: 10) {
            Image(systemName: "externaldrive.connected.to.line.below")
                .font(.system(size: 48))
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(.tint)
                .accessibilityHidden(true)
            Text("Connect to Platrium")
                .font(.title.bold())
            Text(step == .address
                 ? "Enter the address of your Platrium server to get started."
                 : "Sign in to access your files.")
                .font(.callout)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
        }
    }

    private var addressField: some View {
        TextField("Server Address", text: $address, prompt: Text("files.example.com"))
            .focused($addressFocused)
            .autocorrectionDisabled()
            .textContentType(.URL)
            #if os(iOS)
            .keyboardType(.URL)
            .textInputAutocapitalization(.never)
            #endif
            .submitLabel(.continue)
            .onSubmit { Task { await checkServer() } }
            .task { addressFocused = true }
    }

    private var primaryButton: some View {
        Button {
            Task {
                switch step {
                case .address: await checkServer()
                case .signIn(let server, _): await signIn(to: server)
                }
            }
        } label: {
            HStack(spacing: 8) {
                if isWorking { ProgressView().controlSize(.small) }
                Text(step == .address ? "Continue" : "Sign In")
            }
            .frame(maxWidth: platformButtonMaxWidth)
        }
        .keyboardShortcut(.defaultAction)
        .disabled(isWorking || (step == .address && address.trimmingCharacters(in: .whitespaces).isEmpty))
    }

    private var differentServerButton: some View {
        Button("Use a Different Server") {
            errorMessage = nil
            step = .address
        }
        .disabled(isWorking)
    }

    /// iOS buttons span the screen; Mac buttons keep their natural width.
    private var platformButtonMaxWidth: CGFloat? {
        #if os(macOS)
        nil
        #else
        .infinity
        #endif
    }

    // MARK: Actions

    private func checkServer() async {
        guard !isWorking else { return }
        isWorking = true
        errorMessage = nil
        defer { isWorking = false }
        do {
            let (server, details) = try await store.addServer(address: address)
            step = .signIn(server, details)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func signIn(to server: Server) async {
        guard !isWorking else { return }
        isWorking = true
        errorMessage = nil
        defer { isWorking = false }
        do {
            try await store.signIn(to: server, authenticate: webAuthenticationSession.browserAuthenticator)
            store.isShowingAccountSwitcher = false
            if isModal { dismiss() }
        } catch SignInError.cancelled {
            // The user closed the sheet; nothing to report.
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

#Preview {
    ServerSetupView()
        .environment(AccountStore())
}
