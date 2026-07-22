use crate::xplat::file::XPlatFile;
use sha2::{Digest, Sha256};
use tokio::sync::Semaphore;

pub const CHUNK_SIZE_BYTES: u64 = 4 * 1024 * 1024; // 4MB

// Allow at most 5 concurrent chunking tasks to limit memory footprint (5 * 4MB = 20MB max overhead)
static CHUNK_SEMAPHORE: Semaphore = Semaphore::const_new(5);

#[derive(Debug, Clone)]
pub struct ChunkMetadata {
    pub index: usize,
    pub hash: String,
}

pub struct ChunkProcessor<'a> {
    pub xplat_file: &'a XPlatFile,
    pub file_size: u64,
    pub total_chunks: usize,
}

impl<'a> ChunkProcessor<'a> {
    pub fn new(xplat_file: &'a XPlatFile) -> Self {
        let file_size = xplat_file.size();
        let total_chunks = if file_size == 0 {
            0
        } else {
            ((file_size + CHUNK_SIZE_BYTES - 1) / CHUNK_SIZE_BYTES) as usize
        };

        Self {
            xplat_file,
            file_size,
            total_chunks,
        }
    }

    /// Checks if a given chunk index is the final EOF chunk of the file.
    pub fn is_eof_chunk(&self, index: usize) -> bool {
        self.total_chunks > 0 && index == self.total_chunks - 1
    }

    /// Scans a window of chunks starting from `start_index` up to `batch_size`.
    /// Reads 1 chunk at a time into memory, calculates SHA-256 hash, and immediately drops the byte buffer.
    /// Peak memory footprint: ~20MB MAX regardless of file size.
    pub async fn scan_chunk_hashes(
        &self,
        start_index: usize,
        batch_size: usize,
    ) -> Result<Vec<ChunkMetadata>, String> {
        let end_index = (start_index + batch_size).min(self.total_chunks);

        #[cfg(not(target_arch = "wasm32"))]
        {
            let mut handles = Vec::with_capacity(end_index - start_index);

            for idx in start_index..end_index {
                let offset = (idx as u64) * CHUNK_SIZE_BYTES;
                let size = if self.is_eof_chunk(idx) {
                    self.file_size - offset
                } else {
                    CHUNK_SIZE_BYTES
                };

                let xplat_file = self.xplat_file.clone();
                
                handles.push(tokio::spawn(async move {
                    let _permit = CHUNK_SEMAPHORE.acquire().await.map_err(|e| e.to_string())?;
                    let buffer = xplat_file.read_exact_at(offset, size as usize).await?;

                    let hash = tokio::task::spawn_blocking(move || {
                        let mut hasher = Sha256::new();
                        hasher.update(&buffer);
                        format!("{:x}", hasher.finalize())
                    })
                    .await
                    .map_err(|e| format!("Blocking task panicked: {}", e))?;

                    Ok::<ChunkMetadata, String>(ChunkMetadata { index: idx, hash })
                }));
            }

            let mut scanned = Vec::with_capacity(end_index - start_index);
            for handle in handles {
                scanned.push(handle.await.map_err(|e| e.to_string())??);
            }

            Ok(scanned)
        }

        #[cfg(target_arch = "wasm32")]
        {
            let mut scanned = Vec::with_capacity(end_index - start_index);
            for idx in start_index..end_index {
                let offset = (idx as u64) * CHUNK_SIZE_BYTES;
                let size = if self.is_eof_chunk(idx) {
                    self.file_size - offset
                } else {
                    CHUNK_SIZE_BYTES
                };

                let buffer = self.xplat_file.read_exact_at(offset, size as usize).await?;

                let mut hasher = Sha256::new();
                hasher.update(&buffer);
                let hash = format!("{:x}", hasher.finalize());

                scanned.push(ChunkMetadata { index: idx, hash });
            }

            Ok(scanned)
        }
    }

    /// Re-reads the exact byte payload for a specific chunk index on-demand (used for targeted HTTP PUTs or range requests).
    pub async fn read_single_chunk(&self, index: usize) -> Result<Vec<u8>, String> {
        let offset = (index as u64) * CHUNK_SIZE_BYTES;
        let size = if self.is_eof_chunk(index) {
            self.file_size - offset
        } else {
            CHUNK_SIZE_BYTES
        };

        self.xplat_file.read_exact_at(offset, size as usize).await
    }
}
