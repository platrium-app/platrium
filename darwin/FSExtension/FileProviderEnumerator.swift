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
    private let apollo: ApolloClient
    private let anchor = NSFileProviderSyncAnchor("an anchor".data(using: .utf8)!) // TODO: Implement Later
    
    init(enumeratedItemIdentifier: NSFileProviderItemIdentifier, apollo: ApolloClient) {
        self.enumeratedItemIdentifier = enumeratedItemIdentifier
        self.apollo = apollo
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
                    observer.finishEnumeratingWithError(error)
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
                        let items = contents.edges.map { $0.node }.map { FileProviderItem(node: $0) }
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
                    observer.finishEnumeratingWithError(error)
                }
            }
        }
    }
    
    func enumerateChanges(for observer: NSFileProviderChangeObserver, from anchor: NSFileProviderSyncAnchor) {
        // Simply tell macOS there are no changes for now.
        // We will implement real diff syncing later via WebSockets.
        observer.finishEnumeratingChanges(upTo: anchor, moreComing: false)
    }

    func currentSyncAnchor(completionHandler: @escaping (NSFileProviderSyncAnchor?) -> Void) {
        completionHandler(anchor)
    }
}
