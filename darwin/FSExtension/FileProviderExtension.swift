//
//  FileProviderExtension.swift
//  FSExtension
//
//  Created by Atheesh Thirumalairajan on 9/26/26.
//

import FileProvider
import Apollo
import PlatriumCore
import PlatriumGraphQL
import PlatriumSDK
import os
import UniformTypeIdentifiers

private let logger = Logger(subsystem: "org.platrium.FSExtension", category: "Extension")

extension ApolloClient {
    func performAsync<Mutation: GraphQLMutation>(mutation: Mutation) async throws -> Mutation.Data {
        return try await withCheckedThrowingContinuation { continuation in
            self.perform(mutation: mutation) { result in
                switch result {
                case .success(let gqlResult):
                    if let errors = gqlResult.errors, !errors.isEmpty {
                        continuation.resume(throwing: NSError(domain: "FSErrorDomain", code: 1, userInfo: [NSLocalizedDescriptionKey: errors.map { $0.localizedDescription }.joined(separator: ", ")]))
                    } else if let data = gqlResult.data {
                        continuation.resume(returning: data)
                    } else {
                        continuation.resume(throwing: NSError(domain: "FSErrorDomain", code: 1, userInfo: [NSLocalizedDescriptionKey: "No data returned"]))
                    }
                case .failure(let error):
                    continuation.resume(throwing: error)
                }
            }
        }
    }
}

class FileProviderExtension: NSObject, NSFileProviderReplicatedExtension {
    
    let domainId: String
    /// The account behind this domain, or why there isn't one. A domain without
    /// a usable account answers every request with `notAuthenticated`.
    private let sessionResult: Result<DomainSession, Error>

    required init(domain: NSFileProviderDomain) {
        self.domainId = domain.identifier.rawValue
        self.sessionResult = Result { try DomainSession.open(domainIdentifier: domain.identifier.rawValue) }
        logger.info("FileProviderExtension initialized for Domain ID: \(self.domainId, privacy: .public)")

        super.init()
    }

    private func session() throws -> DomainSession {
        try sessionResult.get()
    }

    /// What to hand back to the system for a failure on this domain.
    private func fileProviderError(_ error: Error) -> Error {
        (try? session())?.fileProviderError(error) ?? NSFileProviderError(.notAuthenticated)
    }
    
    func invalidate() {
        // TODO: cleanup any resources
    }
    
    func item(for identifier: NSFileProviderItemIdentifier, request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, Error?) -> Void) -> Progress {
        logger.info("item(for:) called for identifier: \(identifier.rawValue, privacy: .public)")
        let progress = Progress(totalUnitCount: 1)
        
        if identifier == .rootContainer {
            completionHandler(FileProviderItem(identifier: identifier), nil)
            return progress
        }
        
        Task {
            do {
                let info = try await self.fetchItemInfo(id: identifier.rawValue)
                let item = FileProviderItem(fragment: info.fragments.fseDriveItemFields)
                completionHandler(item, nil)
            } catch {
                logger.error("item(for:) failed for \(identifier.rawValue, privacy: .public): \(error.localizedDescription, privacy: .public)")
                completionHandler(nil, (try? self.session())?.mapAuthentication(error) ?? NSFileProviderError(.notAuthenticated))
            }
        }
        
        return progress
    }
    
    /// Fetches a single item's metadata (including parentId) from GraphQL
    private func fetchItemInfo(id: String) async throws -> FSEGetItemInfoQuery.Data.Item {
        let apollo = try session().apollo
        return try await withCheckedThrowingContinuation { continuation in
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
        Task {
            do {
                let utility = try self.session().contentTransfer()
                // Run download and metadata fetch concurrently
                async let downloadResult = utility.download(
                    fileId: itemIdentifier.rawValue,
                    rangeStart: 0,
                    rangeEnd: UInt64.max,
                    progress: progress
                )

                async let info = self.fetchItemInfo(id: itemIdentifier.rawValue)
                let ((url, _), itemInfo) = try await (downloadResult, info)
                let item = FileProviderItem(fragment: itemInfo.fragments.fseDriveItemFields)
                logger.info("fetchContents (full) succeeded for: \(itemInfo.fragments.fseDriveItemFields.name, privacy: .public) size=\(itemInfo.fragments.fseDriveItemFields.asFile?.size ?? "folder", privacy: .public)")
                completionHandler(url, item, nil)
            } catch {
                logger.error("fetchContents (full) failed for: \(itemIdentifier.rawValue, privacy: .public) error: \(error.localizedDescription, privacy: .public)")
                let nsError = self.fileProviderError(error)
                completionHandler(nil, nil, nsError)
            }
        }
        
        return progress
    }

    func createItem(basedOn itemTemplate: NSFileProviderItem, fields: NSFileProviderItemFields, contents url: URL?, options: NSFileProviderCreateItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: -1)
        
        Task {
            do {
                if itemTemplate.contentType == .folder {
                    logger.info("createItem called for folder: \\(itemTemplate.filename, privacy: .public) in parent: \\(itemTemplate.parentItemIdentifier.rawValue, privacy: .public)")
                    
                    let mutation = FSECreateFolderMutation(
                        parentId: itemTemplate.parentItemIdentifier.rawValue,
                        name: itemTemplate.filename
                    )

                    let result = try await self.session().apollo.performAsync(mutation: mutation)
                    guard let newFolderId = result.createFolder.id as String? else {
                        throw NSError(domain: "FSErrorDomain", code: 1, userInfo: [NSLocalizedDescriptionKey: "Failed to parse createFolder response"])
                    }
                    
                    let info = try await self.fetchItemInfo(id: newFolderId)
                    let item = FileProviderItem(fragment: info.fragments.fseDriveItemFields)
                    
                    logger.info("createItem succeeded for folder: \\(info.fragments.fseDriveItemFields.name, privacy: .public) with new ID: \\(newFolderId, privacy: .public)")
                    completionHandler(item, [], false, nil)
                    return
                }
                
                guard let fileUrl = url else {
                    let nsError = NSError(domain: NSCocoaErrorDomain, code: NSFileProviderError.serverUnreachable.rawValue, userInfo: [:])
                    completionHandler(nil, [], false, nsError)
                    return
                }
                
                logger.info("createItem called for file: \(itemTemplate.filename, privacy: .public) in parent: \(itemTemplate.parentItemIdentifier.rawValue, privacy: .public)")
                
                let utility = try self.session().contentTransfer()
                let newFileId = try await utility.upload(
                    parentId: itemTemplate.parentItemIdentifier.rawValue,
                    fileName: itemTemplate.filename,
                    url: fileUrl,
                    progress: progress
                )
                
                let info = try await self.fetchItemInfo(id: newFileId)
                let item = FileProviderItem(fragment: info.fragments.fseDriveItemFields)
                
                logger.info("createItem succeeded for file: \(info.fragments.fseDriveItemFields.name, privacy: .public) with new ID: \(newFileId, privacy: .public)")
                completionHandler(item, [], false, nil)
            } catch {
                logger.error("createItem failed for file: \(itemTemplate.filename, privacy: .public) error: \(error.localizedDescription, privacy: .public)")
                let nsError = self.fileProviderError(error)
                completionHandler(nil, [], false, nsError)
            }
        }
        
        return progress
    }
    
    func modifyItem(_ item: NSFileProviderItem, baseVersion version: NSFileProviderItemVersion, changedFields: NSFileProviderItemFields, contents newContents: URL?, options: NSFileProviderModifyItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 1)
        
        Task {
            do {
                var updatedInfo: FileProviderItem? = nil
                
                if changedFields.contains(.filename) {
                    logger.info("Renaming item: \\(item.itemIdentifier.rawValue) to \\(item.filename)")
                    let mutation = FSERenameItemMutation(id: item.itemIdentifier.rawValue, newName: item.filename)
                    let result = try await self.session().apollo.performAsync(mutation: mutation)
                    
                    if let _ = result.renameItem.id as String? {
                        let info = try await self.fetchItemInfo(id: item.itemIdentifier.rawValue)
                        updatedInfo = FileProviderItem(fragment: info.fragments.fseDriveItemFields)
                    } else {
                        throw NSError(domain: "FSErrorDomain", code: 1, userInfo: [NSLocalizedDescriptionKey: "Failed to parse renameItem response"])
                    }
                }
                
                if changedFields.contains(.parentItemIdentifier) {
                    logger.info("Moving item: \\(item.itemIdentifier.rawValue) to \\(item.parentItemIdentifier.rawValue)")
                    let mutation = FSEMoveItemMutation(id: item.itemIdentifier.rawValue, newParentId: item.parentItemIdentifier.rawValue)
                    let result = try await self.session().apollo.performAsync(mutation: mutation)
                    
                    if let _ = result.moveItem.id as String? {
                        let info = try await self.fetchItemInfo(id: item.itemIdentifier.rawValue)
                        updatedInfo = FileProviderItem(fragment: info.fragments.fseDriveItemFields)
                    } else {
                        throw NSError(domain: "FSErrorDomain", code: 1, userInfo: [NSLocalizedDescriptionKey: "Failed to parse moveItem response"])
                    }
                }
                
                if let finalItem = updatedInfo {
                    completionHandler(finalItem, [], false, nil)
                } else {
                    completionHandler(item, [], false, nil)
                }
            } catch {
                logger.error("modifyItem failed: \\(error.localizedDescription)")
                completionHandler(nil, [], false, (try? self.session())?.mapAuthentication(error) ?? NSFileProviderError(.notAuthenticated))
            }
        }
        
        return progress
    }
    
    func deleteItem(identifier: NSFileProviderItemIdentifier, baseVersion version: NSFileProviderItemVersion, options: NSFileProviderDeleteItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 1)
        
        Task {
            do {
                logger.info("Deleting item: \\(identifier.rawValue)")
                let mutation = FSEDeleteItemMutation(id: identifier.rawValue)
                let _ = try await self.session().apollo.performAsync(mutation: mutation)
                
                completionHandler(nil)
            } catch {
                logger.error("deleteItem failed: \\(error.localizedDescription)")
                completionHandler((try? self.session())?.mapAuthentication(error) ?? NSFileProviderError(.notAuthenticated))
            }
        }
        
        return progress
    }
    
    func enumerator(for containerItemIdentifier: NSFileProviderItemIdentifier, request: NSFileProviderRequest) throws -> NSFileProviderEnumerator {
        return FileProviderEnumerator(enumeratedItemIdentifier: containerItemIdentifier, session: try session())
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
        
        Task {
            do {
                let utility = try self.session().contentTransfer()
                // Run download and metadata fetch concurrently
                async let downloadResult = utility.download(
                    fileId: itemIdentifier.rawValue,
                    rangeStart: UInt64(startByte),
                    rangeEnd: UInt64(alignedEndByte),
                    progress: progress
                )
                
                async let info = self.fetchItemInfo(id: itemIdentifier.rawValue)
                let ((url, actualRangeEnd), itemInfo) = try await (downloadResult, info)
                let item = FileProviderItem(fragment: itemInfo.fragments.fseDriveItemFields)
                // Use actualRangeEnd (clamped to fileSize-1) so we don't report more bytes than we wrote
                let fetchedRange = NSRange(location: startByte, length: Int(actualRangeEnd) - startByte + 1)
                logger.info("fetchPartialContents succeeded for: \(itemInfo.fragments.fseDriveItemFields.name, privacy: .public) fetchedRange=\(startByte)-\(actualRangeEnd)")
                completionHandler(url, item, fetchedRange, [], nil)
            } catch {
                logger.error("fetchPartialContents failed for: \(itemIdentifier.rawValue, privacy: .public) error: \(error.localizedDescription, privacy: .public)")
                let nsError = self.fileProviderError(error)
                completionHandler(nil, nil, requestedRange, [], nsError)
            }
        }
        
        return progress
    }
}
#endif
