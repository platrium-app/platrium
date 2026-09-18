use crate::net::manager::NetworkTransferManager;
use crate::xplat::file::XPlatFile;
use futures::stream::{FuturesUnordered, StreamExt};
use platrium_restapi::apis::configuration::Configuration;
use platrium_restapi::apis::files_api;
use platrium_restapi::models;
use std::sync::Arc;

#[cfg(all(
    not(target_arch = "wasm32"),
    not(target_os = "android"),
    not(target_os = "ios")
))]
use std::fs::File;

#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
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
#[wasm_bindgen::prelude::wasm_bindgen]
impl UploadSource {
    #[wasm_bindgen(constructor)]
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

#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
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

    async fn start_uploadsession(
        &self,
        parent_id: &str,
        file_name: &str,
        xplat: &XPlatFile,
    ) -> Result<String, crate::errors::PlatriumError> {
        let total_size = xplat.size();
        let processor = crate::fs::chunks::ChunkProcessor::new(xplat);

        let transfer_id = uuid::Uuid::new_v4().to_string();

        let cancel_token = self
            .transfer_manager
            .init_transfer(
                &transfer_id,
                crate::net::transfers::TransferDirection::Upload,
                total_size,
                crate::net::transfers::TransferMetadata::FileChunk {
                    folder_id: parent_id.to_string(),
                    file_name: file_name.to_string(),
                },
            )
            .await;

        // Stage 1: Initialize Upload Session
        let init_req = models::FilesUploadSessionInitRequest::new(
            parent_id.to_string(),
            file_name.to_string(),
            total_size as i64,
            "application/octet-stream".to_string(),
        );

        let init_res = match files_api::upload_session_initialize(&self.api_config, init_req).await
        {
            Ok(res) => res,
            Err(e) => {
                let err_msg = format!("Session init error: {:?}", e);
                self.transfer_manager
                    .emit_error(&transfer_id, err_msg.clone())
                    .await;
                return Err(crate::errors::PlatriumError::ApiError(err_msg));
            }
        };

        self.transfer_manager.start_transfer(&transfer_id).await;

        let session_id = init_res.session_id;

        // Stage 2: Batch Window Scanning & Targeted Presign (128 chunks = 512MB max per batch)
        const BATCH_SIZE: usize = 128;
        let mut master_commit_chunks = Vec::with_capacity(processor.total_chunks);

        for start_idx in (0..processor.total_chunks).step_by(BATCH_SIZE) {
            // 1. Pass 1 (Lightweight Hash Scan): Read 1 chunk at a time, compute hash, discard bytes. Max RAM: 4MB.
            let scanned_batch = match processor.scan_chunk_hashes(start_idx, BATCH_SIZE).await {
                Ok(b) => b,
                Err(e) => {
                    self.transfer_manager
                        .emit_error(&transfer_id, e.clone())
                        .await;
                    return Err(crate::errors::PlatriumError::InternalError(e));
                }
            };

            let batch_hashes: Vec<String> = scanned_batch.iter().map(|c| c.hash.clone()).collect();
            let contains_eof_chunk = scanned_batch
                .iter()
                .any(|c| processor.is_eof_chunk(c.index));

            // 2. Request Presign Status for this batch from Storage Manager
            let mut req = models::FilesUploadSessionChunksRequest::new(batch_hashes);
            if contains_eof_chunk {
                req.contains_eof_chunk = Some(true);
            }

            let presign_res =
                match files_api::upload_session_chunks(&self.api_config, &session_id, req).await {
                    Ok(res) => res,
                    Err(e) => {
                        let err_msg = format!("Session chunks error: {:?}", e);
                        self.transfer_manager
                            .emit_error(&transfer_id, err_msg.clone())
                            .await;
                        return Err(crate::errors::PlatriumError::ApiError(err_msg));
                    }
                };

            // 3. Pass 2 (On-Demand Targeted Upload): Re-read ONLY missing chunks for HTTP PUT
            let mut uploads = FuturesUnordered::new();

            for chunk in &scanned_batch {
                let presign_res = &presign_res;
                let client = &self.api_config.client;
                let processor = &processor;
                let transfer_manager = &self.transfer_manager;
                let cancel_token = cancel_token.clone();
                let transfer_id_clone = transfer_id.clone();

                uploads.push(async move {
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
                            .add_transferred_bytes(&transfer_id_clone, chunk_len as u64)
                            .await;
                    } else {
                        // Increment Network Transfer Manager Bytes?
                    }

                    Ok::<(), crate::errors::PlatriumError>(())
                });
            }

            while let Some(res) = uploads.next().await {
                if let Err(e) = res {
                    self.transfer_manager
                        .emit_error(&transfer_id, format!("{:?}", e))
                        .await;
                    return Err(e);
                }
            }

            // Explicitly drop `uploads` to release the borrow on `scanned_batch` before consuming it
            drop(uploads);

            for chunk in scanned_batch {
                let presigned = presign_res.chunks.get(&chunk.hash).ok_or_else(|| {
                    crate::errors::PlatriumError::ApiError(format!(
                        "Missing presigned info for chunk {}",
                        chunk.hash
                    ))
                })?;

                master_commit_chunks.push(models::FilesUploadSessionCommitChunk::new(
                    chunk.hash,
                    presigned.receipt.clone(),
                ));
            }
        }

        // Stage 3: Zero-Read Commit
        let commit_req = models::FilesUploadSessionCommitRequest::new(master_commit_chunks);

        let commit_res =
            match files_api::upload_session_commit(&self.api_config, &session_id, commit_req).await
            {
                Ok(res) => {
                    self.transfer_manager.complete_transfer(&transfer_id).await;
                    res
                }
                Err(e) => {
                    let err_msg = format!("Session commit error: {:?}", e);
                    self.transfer_manager
                        .emit_error(&transfer_id, err_msg.clone())
                        .await;
                    return Err(crate::errors::PlatriumError::ApiError(err_msg));
                }
            };

        Ok(commit_res.file_id)
    }
}

#[cfg(not(target_arch = "wasm32"))]
#[uniffi::export(callback_interface)]
pub trait TransferEventListener: Send + Sync {
    fn on_event(&self, event: crate::net::transfers::NetTransferEvent);
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
        self.start_uploadsession(parent_id, &source.file_name, &source.xplat)
            .await
    }

    /// Cancels a running upload
    pub async fn cancel_upload(&self, client_file_id: String) {
        self.transfer_manager.cancel_transfer(&client_file_id).await;
    }

    /// Subscribes to transfer events natively for Swift / Kotlin / C++.
    pub fn on_transfer_event(&self, listener: Box<dyn TransferEventListener>) {
        let mut rx = self.transfer_manager.subscribe_events();
        tokio::spawn(async move {
            while let Ok(event) = rx.recv().await {
                listener.on_event(event);
            }
        });
    }
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
pub struct TransferSubscription {
    cancel_token: tokio_util::sync::CancellationToken,
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
impl TransferSubscription {
    #[wasm_bindgen(js_name = unsubscribe)]
    pub fn unsubscribe(&self) {
        self.cancel_token.cancel();
    }
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
impl Api {
    /// Uploads a file by chunking, hashing, and registering it with the backend.
    #[wasm_bindgen(js_name = upload)]
    pub async fn upload(
        &self,
        parent_id: &str,
        source: UploadSource,
    ) -> Result<String, wasm_bindgen::JsValue> {
        self.start_uploadsession(parent_id, &source.file_name, &source.xplat)
            .await
            .map_err(|e| wasm_bindgen::JsValue::from_str(&format!("{:?}", e)))
    }

    /// Cancels a running upload
    #[wasm_bindgen(js_name = cancelUpload)]
    pub async fn cancel_upload(&self, transfer_id: String) {
        self.transfer_manager.cancel_transfer(&transfer_id).await;
    }

    /// Listens to transfer events specifically for files with cleanup handle.
    #[wasm_bindgen(js_name = onTransferEvent)]
    pub fn on_transfer_event(&self, callback: js_sys::Function) -> TransferSubscription {
        let mut rx = self.transfer_manager.subscribe_events();
        let cancel_token = tokio_util::sync::CancellationToken::new();
        let token_clone = cancel_token.clone();

        wasm_bindgen_futures::spawn_local(async move {
            loop {
                tokio::select! {
                    _ = token_clone.cancelled() => {
                        break;
                    }
                    res = rx.recv() => {
                        match res {
                            Ok(event) => {
                                if let Ok(js_val) = serde_wasm_bindgen::to_value(&event) {
                                    let _ = callback.call1(&js_sys::global(), &js_val);
                                }
                            }
                            Err(tokio::sync::broadcast::error::RecvError::Closed) => break,
                            Err(tokio::sync::broadcast::error::RecvError::Lagged(_)) => continue,
                        }
                    }
                }
            }
        });

        TransferSubscription { cancel_token }
    }
}
