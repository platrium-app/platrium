//
//  FileProviderExtension.swift
//  FSExtension
//
//  Created by Atheesh Thirumalairajan on 9/26/26.
//

import FileProvider
import Apollo
import PlatriumGraphQL
import PlatriumSDK
import os

private let logger = Logger(subsystem: "org.platrium.FSExtension", category: "Extension")

class FileProviderExtension: NSObject, NSFileProviderReplicatedExtension {
    
    let domainId: String
    let serverId: String
    let userId: String
    let apollo: ApolloClient
    
    required init(domain: NSFileProviderDomain) {
        self.domainId = domain.identifier.rawValue
        
        // TODO: Look these up from a shared SQLite DB using domainId
        self.serverId = "dummy-server-id"
        self.userId = "dummy-user-id"
        
        // Setup GraphQL Client (Fallback Auth for now)
        let serverUrl = URL(string: "http://172.20.0.179:3000/graphql")! // Assume default dev server
        self.apollo = ApolloClient(url: serverUrl)
        logger.info("FileProviderExtension initialized for Domain ID: \(self.domainId, privacy: .public)")
        
        super.init()
    }
    
    func invalidate() {
        // TODO: cleanup any resources
    }
    
    func item(for identifier: NSFileProviderItemIdentifier, request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, Error?) -> Void) -> Progress {
        logger.info("item(for:) called for identifier: \(identifier.rawValue, privacy: .public)")
        // TODO: implement the actual lookup via GraphQL
        completionHandler(FileProviderItem(identifier: identifier), nil)
        return Progress()
    }
    
    /// Fetches a single item's metadata (including parentId) from GraphQL
    private func fetchItemInfo(id: String) async throws -> FSEGetItemInfoQuery.Data.Item {
        try await withCheckedThrowingContinuation { continuation in
            apollo.fetch(query: FSEGetItemInfoQuery(id: id), cachePolicy: .fetchIgnoringCacheData) { result in
                switch result {
                case .success(let gqlResult):
                    if let item = gqlResult.data?.item {
                        continuation.resume(returning: item)
                    } else {
                        continuation.resume(throwing: NSError(domain: NSCocoaErrorDomain, code: NSFileNoSuchFileError))
                    }
                case .failure(let error):
                    continuation.resume(throwing: error)
                }
            }
        }
    }
    
    func fetchContents(for itemIdentifier: NSFileProviderItemIdentifier, version requestedVersion: NSFileProviderItemVersion?, request: NSFileProviderRequest, completionHandler: @escaping (URL?, NSFileProviderItem?, Error?) -> Void) -> Progress {
        logger.info("fetchContents (full) called for: \(itemIdentifier.rawValue, privacy: .public)")
        let progress = Progress(totalUnitCount: -1) // -1 signifies indeterminate progress initially
        let utility = ContentTransferUtility.shared // Access synchronously!
        
        Task {
            do {
                // Run download and metadata fetch concurrently
                async let downloadResult = utility.download(
                    fileId: itemIdentifier.rawValue,
                    rangeStart: 0,
                    rangeEnd: UInt64.max,
                    progress: progress
                )

                async let info = self.fetchItemInfo(id: itemIdentifier.rawValue)
                let ((url, _), itemInfo) = try await (downloadResult, info)
                let item = FileProviderItem(identifier: itemIdentifier, info: itemInfo)
                logger.info("fetchContents (full) succeeded for: \(itemInfo.name, privacy: .public) size=\(itemInfo.asFile?.size ?? "folder", privacy: .public)")
                completionHandler(url, item, nil)
            } catch {
                logger.error("fetchContents (full) failed for: \(itemIdentifier.rawValue, privacy: .public) error: \(error.localizedDescription, privacy: .public)")
                let nsError = NSError(domain: NSCocoaErrorDomain, code: NSFileProviderError.serverUnreachable.rawValue, userInfo: [NSUnderlyingErrorKey: error])
                completionHandler(nil, nil, nsError)
            }
        }
        
        return progress
    }

    func createItem(basedOn itemTemplate: NSFileProviderItem, fields: NSFileProviderItemFields, contents url: URL?, options: NSFileProviderCreateItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {
        // TODO: a new item was created on disk, process the item's creation
        
        completionHandler(itemTemplate, [], false, nil)
        return Progress()
    }
    
    func modifyItem(_ item: NSFileProviderItem, baseVersion version: NSFileProviderItemVersion, changedFields: NSFileProviderItemFields, contents newContents: URL?, options: NSFileProviderModifyItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {
        // TODO: an item was modified on disk, process the item's modification
        
        completionHandler(nil, [], false, NSError(domain: NSCocoaErrorDomain, code: NSFeatureUnsupportedError, userInfo:[:]))
        return Progress()
    }
    
    func deleteItem(identifier: NSFileProviderItemIdentifier, baseVersion version: NSFileProviderItemVersion, options: NSFileProviderDeleteItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (Error?) -> Void) -> Progress {
        // TODO: an item was deleted on disk, process the item's deletion
        
        completionHandler(NSError(domain: NSCocoaErrorDomain, code: NSFeatureUnsupportedError, userInfo:[:]))
        return Progress()
    }
    
    func enumerator(for containerItemIdentifier: NSFileProviderItemIdentifier, request: NSFileProviderRequest) throws -> NSFileProviderEnumerator {
        return FileProviderEnumerator(enumeratedItemIdentifier: containerItemIdentifier, apollo: apollo)
    }
}

#if os(macOS)
extension FileProviderExtension: NSFileProviderPartialContentFetching {
    func fetchPartialContents(for itemIdentifier: NSFileProviderItemIdentifier, version requestedVersion: NSFileProviderItemVersion, request: NSFileProviderRequest, minimalRange requestedRange: NSRange, aligningTo alignment: Int, options: NSFileProviderFetchContentsOptions = [], completionHandler: @escaping (URL?, NSFileProviderItem?, NSRange, NSFileProviderMaterializationFlags, (any Error)?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: Swift.Int64(requestedRange.length))
        
        let startByte = requestedRange.location - (requestedRange.location % alignment)
        let endByte = (requestedRange.location + requestedRange.length + alignment - 1)
        let alignedEndByte = endByte - (endByte % alignment) - 1 // Inclusive end
        
        logger.info("fetchPartialContents called for: \(itemIdentifier.rawValue, privacy: .public) minimalRange=\(requestedRange.location)-\(requestedRange.location + requestedRange.length) alignedRange=\(startByte)-\(alignedEndByte) alignment=\(alignment)")
        
        let utility = ContentTransferUtility.shared // Access synchronously!
        
        Task {
            do {
                // Run download and metadata fetch concurrently
                async let downloadResult = utility.download(
                    fileId: itemIdentifier.rawValue,
                    rangeStart: UInt64(startByte),
                    rangeEnd: UInt64(alignedEndByte),
                    progress: progress
                )
                
                async let info = self.fetchItemInfo(id: itemIdentifier.rawValue)
                let ((url, actualRangeEnd), itemInfo) = try await (downloadResult, info)
                let item = FileProviderItem(identifier: itemIdentifier, info: itemInfo)
                // Use actualRangeEnd (clamped to fileSize-1) so we don't report more bytes than we wrote
                let fetchedRange = NSRange(location: startByte, length: Int(actualRangeEnd) - startByte + 1)
                logger.info("fetchPartialContents succeeded for: \(itemInfo.name, privacy: .public) fetchedRange=\(startByte)-\(actualRangeEnd)")
                completionHandler(url, item, fetchedRange, [], nil)
            } catch {
                logger.error("fetchPartialContents failed for: \(itemIdentifier.rawValue, privacy: .public) error: \(error.localizedDescription, privacy: .public)")
                let nsError = NSError(domain: NSCocoaErrorDomain, code: NSFileProviderError.serverUnreachable.rawValue, userInfo: [NSUnderlyingErrorKey: error])
                completionHandler(nil, nil, requestedRange, [], nsError)
            }
        }
        
        return progress
    }
}
#endif