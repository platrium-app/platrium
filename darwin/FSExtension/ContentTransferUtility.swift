//
//  ContentDownloadUtility.swift
//  Platrium
//
//  Created by Atheesh Thirumalairajan on 9/26/26.
//

import Foundation
import FileProvider
import PlatriumSDK

class ContentTransferUtility {
    static let shared = ContentTransferUtility()
    
    let client: PlatriumClient
    
    // TODO: URL should be looked up from a shared config/DB keyed by domain ID for multi-server support
    private convenience init() {
        self.init(client: try! PlatriumClient(baseUrl: "http://172.20.0.179:3000/api"))
    }
    
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
        
        let listener = TransferProgressListener(progress: progress, sessionId: session.sessionId())
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
    
    // Skeleton for future upload implementation
    func upload(
        itemIdentifier: NSFileProviderItemIdentifier,
        url: URL,
        progress: Progress
    ) async throws {
        let filesApi = client.files()
        // 1. Get file descriptor from URL
        // 2. Create UploadSource
        // 3. Register TransferProgressListener
        // 4. Start upload session
    }
}

final class TransferProgressListener: TransferEventListener {
    let progress: Progress
    let sessionId: String
    
    init(progress: Progress, sessionId: String) {
        self.progress = progress
        self.sessionId = sessionId
    }
    
    func onEvent(event: NetTransferEvent) {
        if event.transferId == sessionId {
            progress.completedUnitCount = Int64(event.bytesTransferred)
        }
    }
}
