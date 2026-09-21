use crate::net::manager::NetworkTransferManager;
use crate::xplat::file::XPlatFile;
#[cfg(target_arch = "wasm32")]
use futures::stream::FuturesOrdered;

#[cfg(not(target_arch = "wasm32"))]
use futures::stream::FuturesUnordered;
use futures::stream::StreamExt;

use platrium_restapi::apis::configuration::Configuration;
use platrium_restapi::apis::files_api;
use platrium_restapi::models;
use std::sync::Arc;

use super::Api;

#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
#[derive(uniffi::Object, Clone)]
pub struct DownloadDestination {
    pub(crate) xplat_file: XPlatFile,
}

#[cfg(not(target_arch = "wasm32"))]
impl DownloadDestination {
    /// Creates a download destination targeting an open File descriptor.
    pub fn new(file: std::fs::File) -> Self {
        Self {
            xplat_file: XPlatFile::new(file),
        }
    }
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
impl DownloadDestination {
    #[wasm_bindgen(constructor)]
    pub fn new(stream: web_sys::WritableStream) -> Self {
        Self {
            xplat_file: XPlatFile::new_write_stream(stream),
        }
    }
}

#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
#[derive(Clone, uniffi::Object)]
pub struct DownloadSession {
    pub(crate) session_id: String,
    pub(crate) file_name: String,
    pub(crate) file_size: u64,
    pub(crate) mime_type: String,
    pub(crate) api_config: Arc<Configuration>,
    pub(crate) transfer_manager: Arc<NetworkTransferManager>,
    pub(crate) http_client: reqwest::Client,
}

#[cfg_attr(not(target_arch = "wasm32"), uniffi::export)]
#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
impl DownloadSession {
    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(getter, js_name = fileName))]
    pub fn file_name(&self) -> String {
        self.file_name.clone()
    }

    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(getter, js_name = fileSize))]
    pub fn file_size(&self) -> u64 {
        self.file_size
    }

    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(getter, js_name = mimeType))]
    pub fn mime_type(&self) -> String {
        self.mime_type.clone()
    }

    /// Streams the entire file to the destination.
    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(js_name = streamTo))]
    pub async fn stream_to(
        &self,
        destination: &DownloadDestination,
    ) -> Result<(), crate::errors::PlatriumError> {
        let range_end_byte = self.file_size.saturating_sub(1);
        self.stream_range_to(destination, 0, range_end_byte).await
    }

    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(js_name = streamRangeTo))]
    pub async fn stream_range_to(
        &self,
        destination: &DownloadDestination,
        range_start_byte: u64,
        range_end_byte: u64,
    ) -> Result<(), crate::errors::PlatriumError> {
        if range_start_byte > range_end_byte || range_end_byte >= self.file_size {
            return Err(crate::errors::PlatriumError::InternalError(
                "Invalid byte range for download stream".to_string(),
            ));
        }

        // 1. Calculate the required chunk indices
        let chunk_indices =
            crate::fs::chunks::byte_range_to_chunk_indices(range_start_byte, range_end_byte);
        let chunks_to_process: Vec<i32> = chunk_indices.map(|i| i as i32).collect();

        // 2. Process chunks in batches of up to 512 (API constraint)
        for chunk_batch in chunks_to_process.chunks(512) {
            let req = models::FilesDownloadSessionChunksRequest::new(chunk_batch.to_vec());

            // Fetch presigned URLs for this batch of chunks
            let presign_res =
                files_api::download_session_chunks(&self.api_config, &self.session_id, req)
                    .await
                    .map_err(|e| {
                        crate::errors::PlatriumError::ApiError(format!(
                            "Failed to fetch chunk presigned URLs: {:?}",
                            e
                        ))
                    })?;

            // Native supports random-access disk writes. We use FuturesUnordered to yield chunks
            // the exact millisecond they finish downloading to instantly write and free memory.
            #[cfg(not(target_arch = "wasm32"))]
            let mut downloads = FuturesUnordered::new();

            // WASM writes to a WritableStream which requires strictly sequential writes.
            // We use FuturesOrdered so out-of-order chunks downloads are automatically buffered and yielded sequentially.
            #[cfg(target_arch = "wasm32")]
            let mut downloads = FuturesOrdered::new();

            for chunk_index in chunk_batch {
                let chunk_index_str = chunk_index.to_string();

                if let Some(presigned_chunk) = presign_res.chunks.get(&chunk_index_str) {
                    let client = self.http_client.clone();
                    let download_url = presigned_chunk.download_url.clone();
                    let transfer_manager = self.transfer_manager.clone();
                    let chunk_idx = *chunk_index as usize;

                    let fut = async move {
                        // 3. Acquire a Global Transfer Slot to prevent network exhaustion
                        let _transfer_slot = transfer_manager.acquire_slot().await;
                        let (chunk_start_offset, chunk_end_offset) =
                            crate::fs::chunks::get_chunk_local_range(
                                range_start_byte,
                                range_end_byte,
                                chunk_idx,
                            );

                        // 4. Execution & Writing
                        let get_req = client.get(&download_url).header(
                            "Range",
                            format!("bytes={}-{}", chunk_start_offset, chunk_end_offset),
                        );

                        let res = get_req.send().await.map_err(|e| {
                            crate::errors::PlatriumError::ApiError(format!(
                                "Chunk GET failed: {:?}",
                                e
                            ))
                        })?;

                        let status = res.status();
                        let bytes = res.bytes().await.map_err(|e| {
                            crate::errors::PlatriumError::ApiError(format!(
                                "Failed to read chunk bytes: {:?}",
                                e
                            ))
                        })?;

                        let payload = match status {
                            reqwest::StatusCode::PARTIAL_CONTENT => bytes,
                            reqwest::StatusCode::OK => {
                                // Backend ignored Range header, manually slice the payload
                                let start = chunk_start_offset as usize;
                                let end = chunk_end_offset as usize;
                                if bytes.len() > end {
                                    bytes.slice(start..=end)
                                } else if bytes.len() > start {
                                    bytes.slice(start..)
                                } else {
                                    bytes.slice(0..0)
                                }
                            }
                            _ => {
                                return Err(crate::errors::PlatriumError::ApiError(format!(
                                    "Chunk GET HTTP {} for chunk {}",
                                    status, chunk_idx
                                )));
                            }
                        };

                        Ok((chunk_idx, chunk_start_offset, payload))
                    };

                    #[cfg(not(target_arch = "wasm32"))]
                    downloads.push(fut);

                    #[cfg(target_arch = "wasm32")]
                    downloads.push_back(fut);
                }
            }

            while let Some(res) = downloads.next().await {
                match res {
                    Ok((_chunk_idx, _chunk_start_offset, payload)) => {
                        if !payload.is_empty() {
                            #[cfg(not(target_arch = "wasm32"))]
                            {
                                // Since Native supports random-access disk writes, we calculate the absolute byte offset
                                // and instantly write the randomly yielded chunk to disk to free memory.
                                let write_offset = (_chunk_idx as u64
                                    * crate::fs::chunks::CHUNK_SIZE_BYTES)
                                    + _chunk_start_offset;

                                destination
                                    .xplat_file
                                    .write_exact_at(write_offset, &payload)
                                    .await
                                    .map_err(|e| {
                                        crate::errors::PlatriumError::InternalError(format!(
                                            "Failed to write chunk: {}",
                                            e
                                        ))
                                    })?;
                            }

                            #[cfg(target_arch = "wasm32")]
                            {
                                // WASM WritableStream requires sequential writes.
                                // FuturesOrdered ensures this block only receives chunks in perfect sequential order.
                                destination
                                    .xplat_file
                                    .write_sequentially(&payload)
                                    .await
                                    .map_err(|e| {
                                        crate::errors::PlatriumError::InternalError(format!(
                                            "Failed to write chunk sequentially: {}",
                                            e
                                        ))
                                    })?;
                            }

                            self.transfer_manager
                                .add_transferred_bytes(&self.session_id, payload.len() as u64)
                                .await;
                        }
                    }
                    Err(e) => {
                        self.transfer_manager
                            .emit_error(&self.session_id, format!("{:?}", e))
                            .await;
                        return Err(e);
                    }
                }
            }
        }

        Ok(())
    }
}

impl Api {
    pub async fn create_download_session(
        &self,
        file_id: String,
    ) -> Result<DownloadSession, crate::errors::PlatriumError> {
        let req = models::FilesDownloadSessionInitRequest::new(file_id);
        let resp = files_api::download_session_initialize(&self.api_config, req)
            .await
            .map_err(|e| crate::errors::PlatriumError::ApiError(e.to_string()))?;

        Ok(DownloadSession {
            session_id: resp.session_id,
            file_name: resp.file_name,
            file_size: resp.file_size as u64,
            mime_type: resp.mime_type,
            api_config: self.api_config.clone(),
            transfer_manager: self.transfer_manager.clone(),
            http_client: reqwest::Client::new(),
        })
    }
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
impl Api {
    /// Creates a download session for a given file ID.
    #[wasm_bindgen(js_name = createDownloadSession)]
    pub async fn create_download_session_wasm(
        &self,
        file_id: String,
    ) -> Result<DownloadSession, wasm_bindgen::JsValue> {
        self.create_download_session(file_id)
            .await
            .map_err(|e| wasm_bindgen::JsValue::from_str(&format!("{:?}", e)))
    }
}
