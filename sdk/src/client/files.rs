use crate::net::manager::NetworkTransferManager;
use crate::xplat::file::XPlatFile;
use futures::stream::StreamExt;
use platrium_restapi::apis::configuration::Configuration;
use platrium_restapi::apis::files_api;
use platrium_restapi::models;
use std::fs::File;
use std::sync::Arc;

#[derive(uniffi::Object)]
pub struct UploadSource {
    pub(crate) file_name: String,
    pub(crate) xplat: XPlatFile,
}

#[cfg(any(target_os = "android", target_os = "ios"))]
#[uniffi::export]
impl UploadSource {
    #[uniffi::constructor]
    pub fn new(file_name: String, fd: i32) -> Self {
        use std::os::unix::io::FromRawFd;
        Self {
            file_name,
            xplat: XPlatFile::new(unsafe { std::fs::File::from_raw_fd(fd) }),
        }
    }
}

#[cfg(target_arch = "wasm32")]
impl UploadSource {
    pub fn new(file_name: String, file: web_sys::File) -> Self {
        Self {
            file_name,
            xplat: XPlatFile::new(file),
        }
    }
}

#[cfg(all(
    not(target_arch = "wasm32"),
    not(target_os = "android"),
    not(target_os = "ios")
))]
#[uniffi::export]
impl UploadSource {
    #[uniffi::constructor]
    pub fn new(file_name: String, path: String) -> Self {
        let file = File::open(path).unwrap();
        Self {
            file_name,
            xplat: XPlatFile::new(file),
        }
    }
}

#[derive(Clone, uniffi::Object)]
pub struct Api {
    api_config: Arc<Configuration>,
    transfer_manager: Arc<NetworkTransferManager>,
}

impl Api {
    // Not exposed to UniFFI because it's crate-internal
    pub(crate) fn new(
        api_config: Arc<Configuration>,
        transfer_manager: Arc<NetworkTransferManager>,
    ) -> Self {
        Self {
            api_config,
            transfer_manager,
        }
    }

    async fn upload_internal(
        &self,
        parent_id: &str,
        file_name: &str,
        xplat: &XPlatFile,
    ) -> Result<String, crate::errors::PlatriumError> {
        let total_size = xplat.size();
        let processor = crate::fs::chunks::ChunkProcessor::new(xplat);

        // Stage 1: Initialize Upload Session
        let init_req = models::FilesUploadSessionInitRequest::new(
            parent_id.to_string(),
            file_name.to_string(),
            total_size as i64,
        );

        let init_res = files_api::upload_session_initialize(&self.api_config, init_req)
            .await
            .map_err(|e| {
                crate::errors::PlatriumError::ApiError(format!("Session init error: {:?}", e))
            })?;

        let session_id = init_res.session_id;

        let cancel_token = self
            .transfer_manager
            .register_transfer(session_id.clone(), total_size)
            .await;

        // Stage 2: Batch Window Scanning & Targeted Presign (500 chunks max per batch)
        const BATCH_SIZE: usize = 500;
        let mut master_commit_chunks = Vec::with_capacity(processor.total_chunks);

        for start_idx in (0..processor.total_chunks).step_by(BATCH_SIZE) {
            // 1. Pass 1 (Lightweight Hash Scan): Read 1 chunk at a time, compute hash, discard bytes. Max RAM: 4MB.
            let scanned_batch = processor
                .scan_chunk_hashes(start_idx, BATCH_SIZE)
                .await
                .map_err(|e| crate::errors::PlatriumError::InternalError(e))?;

            let batch_hashes: Vec<String> = scanned_batch.iter().map(|c| c.hash.clone()).collect();
            let contains_eof_chunk = scanned_batch
                .iter()
                .any(|c| processor.is_eof_chunk(c.index));

            // 2. Request Presign Status for this batch from Storage Manager
            let mut req = models::FilesUploadSessionChunksRequest::new(batch_hashes);
            if contains_eof_chunk {
                req.contains_eof_chunk = Some(true);
            }

            let presign_res = files_api::upload_session_chunks(&self.api_config, &session_id, req)
                .await
                .map_err(|e| {
                    crate::errors::PlatriumError::ApiError(format!("Session chunks error: {:?}", e))
                })?;

            // 3. Pass 2 (On-Demand Targeted Upload): Re-read ONLY missing chunks for HTTP PUT
            let upload_results: Vec<
                Result<models::FilesUploadSessionCommitChunk, crate::errors::PlatriumError>,
            > = futures::stream::iter(scanned_batch)
                .map(|chunk| {
                    let presign_res = &presign_res;
                    let client = &self.api_config.client;
                    let processor = &processor;
                    let transfer_manager = &self.transfer_manager;
                    let cancel_token = cancel_token.clone();
                    let session_id_clone = session_id.clone();

                    async move {
                        let presigned = presign_res.chunks.get(&chunk.hash).ok_or_else(|| {
                            crate::errors::PlatriumError::ApiError(format!(
                                "Missing presigned info for chunk {}",
                                chunk.hash
                            ))
                        })?;

                        if let Some(upload_url) = &presigned.upload_url {
                            // Acquire Global Transfer Slot from Network Transfer Manager
                            let _transfer_slot = transfer_manager.acquire_slot().await;

                            // Re-read ONLY this single chunk's bytes on demand from the file handle!
                            let chunk_bytes = processor
                                .read_single_chunk(chunk.index)
                                .await
                                .map_err(|e| crate::errors::PlatriumError::InternalError(e))?;

                            let chunk_len = chunk_bytes.len();

                            let put_future = client
                                .put(upload_url) // Object Stores have standardized PUT
                                .body(chunk_bytes)
                                .send();

                            let res = tokio::select! {
                                _ = cancel_token.cancelled() => {
                                    return Err(crate::errors::PlatriumError::InternalError(
                                        "Transfer cancelled".into(),
                                    ));
                                }
                                result = put_future => {
                                    result.map_err(|e| {
                                        crate::errors::PlatriumError::ApiError(format!(
                                            "Chunk PUT failed for hash {}: {:?}",
                                            chunk.hash, e
                                        ))
                                    })?
                                }
                            };

                            if !res.status().is_success() {
                                let status = res.status();
                                let err_text = res.text().await.unwrap_or_default();
                                return Err(crate::errors::PlatriumError::ApiError(format!(
                                    "Chunk PUT HTTP {} for hash {}: {}",
                                    status, chunk.hash, err_text
                                )));
                            }

                            transfer_manager
                                .emit_progress(&session_id_clone, chunk_len)
                                .await;
                        }

                        Ok(models::FilesUploadSessionCommitChunk::new(
                            chunk.hash,
                            presigned.receipt.clone(),
                        ))
                    }
                })
                .buffer_unordered(BATCH_SIZE)
                .collect()
                .await;

            for res in upload_results {
                master_commit_chunks.push(res?);
            }
        }

        // Stage 3: Zero-Read Commit
        let commit_req = models::FilesUploadSessionCommitRequest::new(master_commit_chunks);

        let commit_res =
            match files_api::upload_session_commit(&self.api_config, &session_id, commit_req).await
            {
                Ok(res) => {
                    self.transfer_manager.emit_completed(&session_id).await;
                    res
                }
                Err(e) => {
                    let err_msg = format!("Session commit error: {:?}", e);
                    self.transfer_manager
                        .emit_error(&session_id, err_msg.clone())
                        .await;
                    return Err(crate::errors::PlatriumError::ApiError(err_msg));
                }
            };

        Ok(commit_res.file_id)
    }
}

#[cfg(not(target_arch = "wasm32"))]
#[uniffi::export(async_runtime = "tokio")]
impl Api {
    /// Uploads a file by chunking, hashing, and registering it with the backend.
    pub async fn upload(
        &self,
        parent_id: &str,
        source: Arc<UploadSource>,
    ) -> Result<String, crate::errors::PlatriumError> {
        self.upload_internal(parent_id, &source.file_name, &source.xplat)
            .await
    }

    /// Cancels a running upload
    pub async fn cancel_upload(&self, session_id: String) {
        self.transfer_manager.cancel_transfer(&session_id).await;
    }
}
