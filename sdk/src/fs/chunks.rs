use crate::CHUNK_SIZE;
use crate::xplat::file::XPlatFile;
use sha2::{Digest, Sha256};
//TODO: WIP, Add comments, Tests, etc.
pub struct ChunkMetadata {
    pub index: usize,
    pub hash: String,
    pub offset: u64,
    pub size: u64,
}

pub struct ChunkDef {
    pub index: usize,
    pub offset: u64,
    pub size: u64,
}

pub struct ChunkIterator {
    file_size: u64,
    current_index: usize,
    num_chunks: usize,
}

impl ChunkIterator {
    pub fn new(file_size: u64) -> Self {
        let num_chunks = (file_size as f64 / CHUNK_SIZE as f64).ceil() as usize;
        Self {
            file_size,
            current_index: 0,
            num_chunks,
        }
    }
}

impl Iterator for ChunkIterator {
    type Item = ChunkDef;

    fn next(&mut self) -> Option<Self::Item> {
        if self.current_index >= self.num_chunks {
            return None;
        }

        let offset = (self.current_index as u64) * CHUNK_SIZE;
        let size = if offset + CHUNK_SIZE > self.file_size {
            self.file_size - offset
        } else {
            CHUNK_SIZE
        };

        let chunk = ChunkDef {
            index: self.current_index,
            offset,
            size,
        };

        self.current_index += 1;
        Some(chunk)
    }
}

pub async fn hash_chunks(xplat_file: &XPlatFile) -> Result<Vec<ChunkMetadata>, String> {
    let size = xplat_file.size();
    if size == 0 {
        return Ok(Vec::new());
    }

    let mut chunks = Vec::new();
    let iter = ChunkIterator::new(size);

    for chunk_def in iter {
        let buffer = xplat_file
            .read_exact_at(chunk_def.offset, chunk_def.size as usize)
            .await?;
        let mut hasher = Sha256::new();
        hasher.update(&buffer);
        let hash = format!("{:x}", hasher.finalize());

        chunks.push(ChunkMetadata {
            index: chunk_def.index,
            hash,
            offset: chunk_def.offset,
            size: chunk_def.size,
        });
    }

    Ok(chunks)
}
