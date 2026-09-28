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
        if itemIdentifier == .rootContainer {
            // This is the Root Container. We shouldn't be able to move files, add new ones,
            // etc. in them, beside the Drives. This folder exists to mount the Drives.
            return [.allowsReading]
        }
        
        if parentItemIdentifier == .rootContainer {
            // These are the Drives themselves. We can read, create and move into them
            // but we cannot rename, move, or delete the Drive folder itself.
            return [.allowsReading, .allowsAddingSubItems, .allowsContentEnumerating]
        }

        // TODO: Set based on file permissions from Platrium Sharing
        return [.allowsReading, .allowsWriting, .allowsRenaming, .allowsDeleting, .allowsAddingSubItems, .allowsReparenting, .allowsContentEnumerating]
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
    init(fragment: FseDriveItemFields) {
        self.itemIdentifier = NSFileProviderItemIdentifier(fragment.id)
        self.parentItemIdentifier = fragment.parentId.map { NSFileProviderItemIdentifier($0) } ?? .rootContainer
        self.filename = fragment.name
        
        let ext = (fragment.name as NSString).pathExtension
        let backendMimeType = fragment.asFile?.mimeType
        
        if fragment.type == .folder {
            self.contentType = .folder
            self.documentSize = nil
        } else {
            if let mime = backendMimeType, let type = UTType(mimeType: mime) {
                self.contentType = type
            } else if !ext.isEmpty, let type = UTType(filenameExtension: ext) {
                self.contentType = type
            } else {
                self.contentType = .data
            }
            
            if let file = fragment.asFile, let size = Double(file.size) {
                self.documentSize = NSNumber(value: size)
            } else {
                self.documentSize = NSNumber(value: 0) // DO NOT REVERT THIS! macOS needs this to display the file!
            }
        }
        
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        self.creationDate = formatter.date(from: fragment.createdAt)
        self.contentModificationDate = formatter.date(from: fragment.updatedAt)
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
}
