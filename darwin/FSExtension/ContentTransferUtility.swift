//
//  ContentDownloadUtility.swift
//  Platrium
//
//  Created by Atheesh Thirumalairajan on 9/26/26.
//

import Foundation
import FileProvider
import PlatriumSDK

/// Moves file contents for one account. Build it from `DomainSession.contentTransfer()`,
/// which hands it a client carrying the account's current token.
class ContentTransferUtility {
    let client: PlatriumClient
    
    init(client: PlatriumClient) {
        self.client = client
    }
    
    func download(
        fileId: String,
        rangeStart: UInt64,
        rangeEnd: UInt64,
        progress: Progress
    ) async throws -> (url: URL, actualRangeEnd: UInt64) {
        let filesApi = client.files()
        let session = try await filesApi.createDownloadSession(fileId: fileId)
        
        if progress.totalUnitCount <= 0 {
            progress.totalUnitCount = Int64(session.fileSize())
        }
        
        let actualEnd = min(rangeEnd, session.fileSize() > 0 ? session.fileSize() - 1 : 0)
        
        let listener = TransferProgressListener(progress: progress, targetFileId: fileId)
        filesApi.onTransferEvent(listener: listener)
        
        let tempUrl = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        FileManager.default.createFile(atPath: tempUrl.path, contents: nil, attributes: nil)
        
        let fileHandle = try FileHandle(forWritingTo: tempUrl)
        let fd = fileHandle.fileDescriptor
        let destination = DownloadDestination(fd: fd)
        
        try await session.streamRangeTo(destination: destination, rangeStartByte: rangeStart, rangeEndByte: actualEnd)
        
        try fileHandle.close()
        return (tempUrl, actualEnd)
    }
    
    func upload(
        parentId: String,
        fileName: String,
        url: URL,
        progress: Progress
    ) async throws -> String {
        let filesApi = client.files()
        
        let fileHandle = try FileHandle(forReadingFrom: url)
        let fd = fileHandle.fileDescriptor
        
        let fileSize = try FileManager.default.attributesOfItem(atPath: url.path)[.size] as? Int64 ?? 0
        if progress.totalUnitCount <= 0 {
            progress.totalUnitCount = fileSize
        }
        
        let listener = TransferProgressListener(progress: progress, targetFileName: fileName, targetFolderId: parentId)
        filesApi.onTransferEvent(listener: listener)
        
        let source = UploadSource(fileName: fileName, fd: fd)
        let newFileId = try await filesApi.upload(parentId: parentId, source: source)
        
        try fileHandle.close()
        return newFileId
    }
}

final class TransferProgressListener: TransferEventListener {
    let progress: Progress
    let targetFileName: String?
    let targetFolderId: String?
    let targetFileId: String?
    
    init(progress: Progress, targetFileName: String? = nil, targetFolderId: String? = nil, targetFileId: String? = nil) {
        self.progress = progress
        self.targetFileName = targetFileName
        self.targetFolderId = targetFolderId
        self.targetFileId = targetFileId
    }
    
    func onEvent(event: NetTransferEvent) {
        switch event.metadata {
        case let .fileUploadEvent(folderId, fileName):
            if folderId == targetFolderId && fileName == targetFileName {
                progress.completedUnitCount = Int64(event.bytesTransferred)
            }
        case let .fileDownloadEvent(fileId):
            if fileId == targetFileId {
                progress.completedUnitCount = Int64(event.bytesTransferred)
            }
        default:
            break
        }
    }
}
