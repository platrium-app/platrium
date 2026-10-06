import Foundation
import Observation
import PlatriumCore

/// State and actions behind the sharing sheet. The server decides everything
/// (which roles exist, who may share, what is valid); this only asks and shows.
@MainActor
@Observable
final class ShareViewModel {
    enum Phase: Equatable {
        case loading
        /// The user may not see or change who has access.
        case denied
        case failed(String)
        case ready
    }

    let target: ShareTarget
    private let service: SharingService

    private(set) var phase: Phase = .loading
    private(set) var snapshot: SharingSnapshot?
    /// True while a change is being sent, so every control can switch off.
    private(set) var isBusy = false
    var errorMessage: String?

    // Adding people
    var query = ""
    private(set) var results: [DirectoryPerson] = []
    private(set) var isSearching = false
    private(set) var recipients: [DirectoryPerson] = []
    var newRole = ""
    var newExpiryEnabled = false
    var newExpiry = ShareViewModel.defaultExpiry

    /// Tomorrow: an expiry in the past or today makes no sense.
    static var defaultExpiry: Date { Calendar.current.date(byAdding: .day, value: 1, to: Date()) ?? Date() }
    static var earliestExpiry: Date { defaultExpiry }

    init(target: ShareTarget, service: SharingService) {
        self.target = target
        self.service = service
    }

    // MARK: Derived

    var access: ItemAccess? { snapshot?.access }
    var roles: [RoleOption] { snapshot?.roles ?? [] }
    var levels: [AccessLevelOption] { snapshot?.levels ?? [] }
    var general: GeneralAccess? { access?.general }

    func roleLabel(_ role: String) -> String {
        roles.first { $0.role == role }?.label ?? SharingFormat.humanizeRole(role)
    }

    func roleOption(_ role: String?) -> RoleOption? {
        guard let role else { return nil }
        return roles.first { $0.role == role }
    }

    /// The level definition for the current general access. When the server no
    /// longer offers it (say public sharing was switched off), it is still shown.
    var currentLevel: AccessLevelOption? {
        guard let general else { return nil }
        return levels.first { $0.level == general.level }
            ?? AccessLevelOption(level: general.level, label: SharingFormat.humanizeRole(general.level), blurb: "", roles: general.role.map { [$0] } ?? [], supportsExpiry: general.expiresAt != nil)
    }

    // MARK: Loading

    func load() async {
        phase = .loading
        do {
            let loaded = try await service.load(itemId: target.id)
            snapshot = loaded
            if newRole.isEmpty { newRole = loaded.roles.first?.role ?? "" }
            phase = .ready
        } catch let error as SharingError {
            phase = error.isDenied ? .denied : .failed(error.localizedDescription)
        } catch {
            phase = .failed(error.localizedDescription)
        }
    }

    // MARK: Adding people

    /// Looks people up after a short pause, once there are two characters to go on.
    func search() async {
        let text = query.trimmingCharacters(in: .whitespaces)
        guard text.count >= 2 else {
            results = []
            isSearching = false
            return
        }
        isSearching = true
        try? await Task.sleep(for: .milliseconds(200))
        guard !Task.isCancelled else { return }
        do {
            let found = try await service.search(text, excludingAccessTo: target.id)
            guard !Task.isCancelled else { return }
            results = found.filter { person in !recipients.contains { $0.id == person.id } }
        } catch {
            guard !Task.isCancelled else { return }
            results = []
        }
        isSearching = false
    }

    func add(_ person: DirectoryPerson) {
        guard !recipients.contains(where: { $0.id == person.id }) else { return }
        recipients.append(person)
        results.removeAll { $0.id == person.id }
        query = ""
    }

    func remove(recipient person: DirectoryPerson) {
        recipients.removeAll { $0.id == person.id }
    }

    /// Shares with everyone chosen, one at a time. Anyone it fails for stays
    /// chosen, with the server's reason shown.
    func shareWithRecipients() async {
        guard !recipients.isEmpty, !newRole.isEmpty else { return }
        let expiry = newExpiryEnabled ? SharingFormat.endOfDay(newExpiry) : nil
        await run {
            var failures: [DirectoryPerson] = []
            var lastError: Error?
            for person in self.recipients {
                do {
                    try await self.service.share(
                        itemId: self.target.id, subjectType: person.type, subjectId: person.personId,
                        role: self.newRole, noDownload: false, expiresAt: expiry
                    )
                } catch {
                    failures.append(person)
                    lastError = error
                }
            }
            self.recipients = failures
            if failures.isEmpty {
                self.newExpiryEnabled = false
                self.newExpiry = Self.defaultExpiry
            }
            if let lastError { throw lastError }
        }
    }

    // MARK: People with access

    /// Changing a role is sharing again with the same person. Their expiry stays;
    /// "can't download" stays only if the new role can still have it.
    func changeRole(of grant: AccessGrant, to role: String) async {
        guard role != grant.role else { return }
        let keepsNoDownload = grant.noDownload && (roleOption(role)?.downloadOptional ?? false)
        await run {
            try await self.service.share(
                itemId: self.target.id, subjectType: grant.subjectType, subjectId: grant.subjectId,
                role: role, noDownload: keepsNoDownload, expiresAt: grant.expiresAt
            )
        }
    }

    func revoke(_ grant: AccessGrant) async {
        await run { try await self.service.revoke(grantId: grant.id) }
    }

    // MARK: General access

    /// Switching level keeps the role if the new level allows it, else takes its
    /// first; "can't download" and the expiry carry over only where they still apply.
    func setLevel(_ level: AccessLevelOption) async {
        guard let current = general, current.level != level.level else { return }
        await run {
            guard level.takesRole else {
                try await self.service.setGeneralAccess(itemId: self.target.id, level: level.level, role: nil, noDownload: false, expiresAt: nil)
                return
            }
            let role = current.role.flatMap { level.roles.contains($0) ? $0 : nil } ?? level.roles[0]
            let keepsNoDownload = current.noDownload && (self.roleOption(role)?.downloadOptional ?? false)
            try await self.service.setGeneralAccess(
                itemId: self.target.id, level: level.level, role: role,
                noDownload: keepsNoDownload, expiresAt: level.supportsExpiry ? current.expiresAt : nil
            )
        }
    }

    func setGeneralRole(_ role: String) async {
        guard let current = general, let level = currentLevel, role != current.role else { return }
        let keepsNoDownload = current.noDownload && (roleOption(role)?.downloadOptional ?? false)
        await run {
            try await self.service.setGeneralAccess(
                itemId: self.target.id, level: current.level, role: role,
                noDownload: keepsNoDownload, expiresAt: level.supportsExpiry ? current.expiresAt : nil
            )
        }
    }

    func setGeneralDownloads(allowed: Bool) async {
        guard let current = general else { return }
        await run {
            try await self.service.setGeneralAccess(
                itemId: self.target.id, level: current.level, role: current.role,
                noDownload: !allowed, expiresAt: current.expiresAt
            )
        }
    }

    func setGeneralExpiry(_ date: Date?) async {
        guard let current = general else { return }
        await run {
            try await self.service.setGeneralAccess(
                itemId: self.target.id, level: current.level, role: current.role,
                noDownload: current.noDownload, expiresAt: date.map { SharingFormat.endOfDay($0) }
            )
        }
    }

    // MARK: Inheritance

    func setInheritance(_ inherit: Bool) async {
        await run { try await self.service.setInheritance(itemId: self.target.id, inherit: inherit) }
    }

    // MARK: Plumbing

    /// Runs one change, then refreshes who has access. Controls are disabled
    /// while it runs; a failure shows the server's reason.
    private func run(_ work: @escaping () async throws -> Void) async {
        guard !isBusy else { return }
        isBusy = true
        errorMessage = nil
        defer { isBusy = false }
        var failure: Error?
        do { try await work() } catch { failure = error }
        // Refresh even after a failure: a partial change may have gone through.
        if let fresh = try? await service.access(itemId: target.id), var current = snapshot {
            current.access = fresh
            snapshot = current
        }
        if let failure { errorMessage = failure.localizedDescription }
    }
}
