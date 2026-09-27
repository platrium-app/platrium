//
//  FileProviderItem.swift
//  FSExtension
//
//  Created by Atheesh Thirumalairajan on 9/26/26.
//

import FileProvider
import UniformTypeIdentifiers
import PlatriumGraphQL

class FileProviderItem: NSObject, NSFileProviderItem {

    let itemIdentifier: NSFileProviderItemIdentifier
    let parentItemIdentifier: NSFileProviderItemIdentifier
    let filename: String
    let contentType: UTType
    let documentSize: NSNumber?
    let creationDate: Date?
    let contentModificationDate: Date?
    
    var capabilities: NSFileProviderItemCapabilities {
        return [.allowsReading] // Keep simple for now
    }
    
    var itemVersion: NSFileProviderItemVersion {
        // TODO: In a real app, this should be a hash of the updatedAt timestamp or similar
        NSFileProviderItemVersion(
            contentVersion: (contentModificationDate?.description ?? "v1").data(using: .utf8)!,
            metadataVersion: (contentModificationDate?.description ?? "v1").data(using: .utf8)!
        )
    }
    
    // Init for Drive (Root level)
    init(drive: FSEGetDomainRootQuery.Data.DriveNode) {
        self.itemIdentifier = NSFileProviderItemIdentifier(drive.id)
        self.parentItemIdentifier = .rootContainer
        self.filename = drive.name
        self.contentType = .folder
        self.documentSize = nil
        self.creationDate = nil
        self.contentModificationDate = nil
        super.init()
    }
    
    // Init for Folder/File Content
    init(node: FSEGetFolderContentsQuery.Data.FolderContents.Edge.Node) {
        self.itemIdentifier = NSFileProviderItemIdentifier(node.id)
        // If parentId is not available, we assume it's attached to the root container (a drive's root)
        self.parentItemIdentifier = node.parentId.map { NSFileProviderItemIdentifier($0) } ?? .rootContainer
        self.filename = node.name
        
        if node.type == .folder {
            self.contentType = .folder
            self.documentSize = nil
        } else {
            // 1. Try to resolve UTType from the backend MIME type
            // 2. Fallback to resolving from the filename extension
            // 3. Fallback to generic data
            let ext = (node.name as NSString).pathExtension
            let backendMimeType = node.asFile?.mimeType
            
            if let mime = backendMimeType, let type = UTType(mimeType: mime) {
                self.contentType = type
            } else if !ext.isEmpty, let type = UTType(filenameExtension: ext) {
                self.contentType = type
            } else {
                self.contentType = .data
            }
            
            if let file = node.asFile, let size = Double(file.size) {
                self.documentSize = NSNumber(value: size)
            } else {
                self.documentSize = NSNumber(value: 0) // DO NOT REVERT THIS! macOS needs this to display the file!
            }
        }
        
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        self.creationDate = formatter.date(from: node.createdAt)
        self.contentModificationDate = formatter.date(from: node.updatedAt)
        super.init()
    }
    
    init(identifier: NSFileProviderItemIdentifier) {
        self.itemIdentifier = identifier
        self.parentItemIdentifier = .rootContainer
        self.filename = identifier.rawValue
        self.contentType = identifier == .rootContainer ? .folder : .data
        self.documentSize = nil
        self.creationDate = nil
        self.contentModificationDate = nil
        super.init()
    }
    
    // Init after a download — uses item metadata from GraphQL so filename, size, and parent are all correct
    init(identifier: NSFileProviderItemIdentifier, info: FSEGetItemInfoQuery.Data.Item) {
        self.itemIdentifier = identifier
        self.parentItemIdentifier = info.parentId.map { NSFileProviderItemIdentifier($0) } ?? .rootContainer
        self.filename = info.name
        
        let ext = (info.name as NSString).pathExtension
        let mimeType = info.asFile?.mimeType
        if let mime = mimeType, let type = UTType(mimeType: mime) {
            self.contentType = type
        } else if !ext.isEmpty, let type = UTType(filenameExtension: ext) {
            self.contentType = type
        } else {
            self.contentType = .data
        }
        
        if let file = info.asFile, let size = Double(file.size) {
            self.documentSize = NSNumber(value: size)
        } else {
            self.documentSize = nil
        }
        
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        self.creationDate = formatter.date(from: info.createdAt)
        self.contentModificationDate = formatter.date(from: info.updatedAt)
        super.init()
    }
}
