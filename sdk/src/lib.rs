pub mod client;
pub(crate) mod fs;
pub(crate) mod xplat;

pub const CHUNK_SIZE: u64 = 4 * 1024 * 1024; // 4MB
