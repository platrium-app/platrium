//
//  FileProviderEnumerator.swift
//  FSExtension
//
//  Created by Atheesh Thirumalairajan on 9/26/26.
//

import FileProvider
import Apollo
import PlatriumGraphQL
import os

private let logger = Logger(subsystem: "org.platrium.FSExtension", category: "Enumerator")

class FileProviderEnumerator: NSObject, NSFileProviderEnumerator {
    
    private let enumeratedItemIdentifier: NSFileProviderItemIdentifier
    private let session: DomainSession
    private var apollo: ApolloClient { session.apollo }
    
    init(enumeratedItemIdentifier: NSFileProviderItemIdentifier, session: DomainSession) {
        self.enumeratedItemIdentifier = enumeratedItemIdentifier
        self.session = session
        super.init()
    }

    func invalidate() {
        // TODO: perform invalidation of server connection if necessary
    }

    func enumerateItems(for observer: NSFileProviderEnumerationObserver, startingAt page: NSFileProviderPage) {
        if enumeratedItemIdentifier == .rootContainer {
            // 1. Root Level: Fetch all available drives
            apollo.fetch(query: FSEGetDomainRootQuery(), cachePolicy: .fetchIgnoringCacheData) { result in
                switch result {
                case .success(let graphQLResult):
                    if let drives = graphQLResult.data?.driveNodes {
                        let items = drives.compactMap { $0 }.map { FileProviderItem(drive: $0) }
                        logger.info("FSExtension Yielding \(items.count) Drives: \(items.map { $0.filename }, privacy: .public)")
                        observer.didEnumerate(items)
                        observer.finishEnumerating(upTo: nil)
                    } else {
                        logger.error("FSExtension Root Query returned 200 OK but Data was nil! GraphQL Errors: \(String(describing: graphQLResult.errors), privacy: .public)")
                        observer.finishEnumeratingWithError(NSError(domain: NSCocoaErrorDomain, code: NSFileNoSuchFileError, userInfo: nil))
                    }
                case .failure(let error):
                    logger.error("FSExtension Root Query Error: \(error.localizedDescription, privacy: .public)")
                    observer.finishEnumeratingWithError(self.session.mapAuthentication(error))
                }
            }
        } else {
            // 2. Folder Level: Fetch contents of a specific folder or drive
            let folderId = enumeratedItemIdentifier.rawValue

            // Compare raw bytes against Apple's real sentinel constants (bridged NSData -> Data).
            // Anything else is treated as OUR own opaque GraphQL cursor.
            let isInitialPage = page.rawValue == (NSFileProviderPage.initialPageSortedByName as Data)
                              || page.rawValue == (NSFileProviderPage.initialPageSortedByDate as Data)

            let cursor: String? = isInitialPage ? nil : String(data: page.rawValue, encoding: .utf8)

            if !isInitialPage && cursor == nil {
                logger.error("FSExtension: non-sentinel page failed to decode as our cursor (\(page.rawValue.count) bytes) — treating as initial page anyway")
            }

            logger.info("FSExtension enumerateItems for folder \(folderId, privacy: .public), page: \(isInitialPage ? "initialPage" : (cursor ?? "raw-undecoded"), privacy: .public)")

            let afterNullable: GraphQLNullable<String> = cursor.map { .some($0) } ?? .none

            apollo.fetch(query: FSEGetFolderContentsQuery(folderId: folderId, first: 50, after: afterNullable), cachePolicy: .fetchIgnoringCacheData) { result in
                switch result {
                case .success(let graphQLResult):
                    if let contents = graphQLResult.data?.folderContents {
                        let items = contents.edges.map { $0.node.fragments.fseDriveItemFields }.map { FileProviderItem(fragment: $0) }
                        logger.info("FSExtension Yielding \(items.count) items to macOS. Names: \(items.map { $0.filename }, privacy: .public)")
                        observer.didEnumerate(items)

                        if contents.pageInfo.hasNextPage,
                           let endCursor = contents.pageInfo.endCursor,
                           let cursorData = endCursor.data(using: .utf8) {
                            observer.finishEnumerating(upTo: NSFileProviderPage(cursorData))
                        } else {
                            observer.finishEnumerating(upTo: nil)
                        }
                    } else {
                        logger.error("FSExtension Folder Query returned 200 OK but Data was nil! GraphQL Errors: \(String(describing: graphQLResult.errors), privacy: .public)")
                        observer.finishEnumeratingWithError(NSError(domain: NSCocoaErrorDomain, code: NSFileNoSuchFileError, userInfo: nil))
                    }
                case .failure(let error):
                    logger.error("FSExtension Folder Query Error: \(error.localizedDescription, privacy: .public)")
                    observer.finishEnumeratingWithError(self.session.mapAuthentication(error))
                }
            }
        }
    }
    
    func enumerateChanges(for observer: NSFileProviderChangeObserver, from anchor: NSFileProviderSyncAnchor) {
        if enumeratedItemIdentifier == .rootContainer {
            enumerateRootChanges(for: observer, from: anchor)
        } else {
            enumerateFolderChanges(for: observer, from: anchor)
        }
    }
    
    /// Syncs changes for the Root Container (Drives). Since the number of drives is small,
    /// we fetch the entire list, diff it against the IDs stored in the anchor, and update macOS.
    private func enumerateRootChanges(for observer: NSFileProviderChangeObserver, from anchor: NSFileProviderSyncAnchor) {
        apollo.fetch(query: FSEGetDomainRootQuery(), cachePolicy: .fetchIgnoringCacheData) { result in
            switch result {
            case .success(let graphQLResult):
                if let drives = graphQLResult.data?.driveNodes {
                    let items = drives.compactMap { $0 }.map { FileProviderItem(drive: $0) }
                    let currentDriveIds = items.map { $0.itemIdentifier.rawValue }
                    
                    let previousDriveIds = (try? JSONDecoder().decode([String].self, from: anchor.rawValue)) ?? []
                    
                    let currentSet = Set(currentDriveIds)
                    let previousSet = Set(previousDriveIds)
                    
                    let deletedIds = previousSet.subtracting(currentSet)
                    
                    if !deletedIds.isEmpty {
                        observer.didDeleteItems(withIdentifiers: deletedIds.map { NSFileProviderItemIdentifier($0) })
                    }
                    
                    // We can just pass all current drives to didUpdate; macOS ignores items that haven't changed.
                    if !items.isEmpty {
                        observer.didUpdate(items)
                    }
                    
                    if let newAnchorData = try? JSONEncoder().encode(currentDriveIds) {
                        observer.finishEnumeratingChanges(upTo: NSFileProviderSyncAnchor(newAnchorData), moreComing: false)
                    } else {
                        observer.finishEnumeratingChanges(upTo: anchor, moreComing: false)
                    }
                } else {
                    observer.finishEnumeratingChanges(upTo: anchor, moreComing: false)
                }
            case .failure(let error):
                logger.error("FSExtension Root GetChanges Error: \(error.localizedDescription, privacy: .public)")
                observer.finishEnumeratingWithError(self.session.mapAuthentication(error))
            }
        }
    }
    
    /// Syncs changes for a specific Folder. We hit the getChanges GraphQL query which returns
    /// a differential sync of created, updated, and deleted items since the given ISO8601 anchor.
    private func enumerateFolderChanges(for observer: NSFileProviderChangeObserver, from anchor: NSFileProviderSyncAnchor) {
        let folderId = enumeratedItemIdentifier.rawValue
        
        // Parse the anchor. It should be an ISO8601 string. If not, default to 1970.
        let sinceStr = String(data: anchor.rawValue, encoding: .utf8) ?? "1970-01-01T00:00:00Z"
        
        apollo.fetch(query: FSEGetChangesQuery(folderId: folderId, since: sinceStr), cachePolicy: .fetchIgnoringCacheData) { result in
            switch result {
            case .success(let graphQLResult):
                if let changes = graphQLResult.data?.getChanges {
                    var updatedItems: [FileProviderItem] = []
                    var deletedItems: [NSFileProviderItemIdentifier] = []
                    
                    for change in changes {
                        if change.eventType == .deleted, let deletedId = change.deletedId {
                            deletedItems.append(NSFileProviderItemIdentifier(deletedId))
                        } else if let item = change.item {
                            updatedItems.append(FileProviderItem(fragment: item.fragments.fseDriveItemFields))
                        }
                    }
                    
                    if !deletedItems.isEmpty {
                        observer.didDeleteItems(withIdentifiers: deletedItems)
                    }
                    
                    if !updatedItems.isEmpty {
                        observer.didUpdate(updatedItems)
                    }
                    
                    // Create a new anchor for the current time to catch any new changes
                    let formatter = ISO8601DateFormatter()
                    formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
                    let newAnchorData = formatter.string(from: Date()).data(using: .utf8)!
                    
                    observer.finishEnumeratingChanges(upTo: NSFileProviderSyncAnchor(newAnchorData), moreComing: false)
                } else {
                    observer.finishEnumeratingChanges(upTo: anchor, moreComing: false)
                }
            case .failure(let error):
                logger.error("FSExtension GetChanges Error: \(error.localizedDescription, privacy: .public)")
                observer.finishEnumeratingWithError(self.session.mapAuthentication(error))
            }
        }
    }

    func currentSyncAnchor(completionHandler: @escaping (NSFileProviderSyncAnchor?) -> Void) {
        if enumeratedItemIdentifier == .rootContainer {
            let emptyArray: [String] = []
            if let data = try? JSONEncoder().encode(emptyArray) {
                completionHandler(NSFileProviderSyncAnchor(data))
            } else {
                completionHandler(nil)
            }
            return
        }
        
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let anchorData = formatter.string(from: Date()).data(using: .utf8)!
        completionHandler(NSFileProviderSyncAnchor(anchorData))
    }
}
