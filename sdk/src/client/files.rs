use crate::fs::chunks::hash_chunks;
use crate::xplat::file::XPlatFile;
use platrium_restapi::apis::configuration::Configuration;
use platrium_restapi::apis::files_api;
use platrium_restapi::models::FilesCreateFileRequest;
use std::sync::Arc;
//TODO Needs better comments showing UPloadSource abstracttion etc
#[cfg(any(target_os = "android", target_os = "ios", target_arch = "wasm32"))]
pub struct UploadSource {
    pub file_name: String,
    pub(crate) xplat: XPlatFile, // bruh this needs to be an FD or smth? better design buggy rn.
}

#[cfg(any(target_os = "android", target_os = "ios"))]
impl UploadSource {
    pub fn new(file_name: String, fd: std::os::unix::io::RawFd) -> Self {
        use std::os::unix::io::FromRawFd;
        Self {
            file_name,
            xplat: XPlatFile::new_native(unsafe { std::fs::File::from_raw_fd(fd) }),
        }
    }
}

#[cfg(target_arch = "wasm32")]
impl UploadSource {
    pub fn new(file_name: String, file: web_sys::File) -> Self {
        Self {
            file_name,
            xplat: XPlatFile::new_wasm(file),
        }
    }
}

#[derive(Clone)]
pub struct FilesClient {
    api_config: Arc<Configuration>,
}

impl FilesClient {
    pub(crate) fn new(api_config: Arc<Configuration>) -> Self {
        Self { api_config }
    }

    async fn upload_internal(
        &self,
        parent_id: &str,
        file_name: &str,
        xplat: &XPlatFile,
    ) -> Result<String, String> {
        // 1. Hash the chunks locally
        let chunks = hash_chunks(xplat)
            .await
            .map_err(|e| format!("Failed to hash file: {}", e))?;

        let hashes: Vec<String> = chunks.into_iter().map(|c| c.hash).collect();
        println!("Chunks: {:?}", hashes);

        // 2. Call the REST API to register the file and get the missing chunks
        let req = FilesCreateFileRequest {
            parent_id: parent_id.to_string(),
            file_name: file_name.to_string(),
            hashes,
        };

        // TODO: Handle the 404 MissingChunks Error and actually upload the bytes!
        let response = files_api::files_create_file(&self.api_config, req)
            .await
            .map_err(|e| format!("API Error: {:?}", e))?;

        Ok(response.file_id)
    }

    /// Uploads a file by chunking, hashing, and registering it with the backend.
    #[cfg(any(target_os = "android", target_os = "ios", target_arch = "wasm32"))]
    pub async fn upload(&self, parent_id: &str, source: &UploadSource) -> Result<String, String> {
        self.upload_internal(parent_id, &source.file_name, &source.xplat)
            .await
    }

    /// Uploads a file by chunking, hashing, and registering it with the backend.
    #[cfg(all(
        not(target_arch = "wasm32"),
        not(target_os = "android"),
        not(target_os = "ios")
    ))]
    pub async fn upload(
        &self,
        parent_id: &str,
        file_name: &str,
        file_path: &str,
    ) -> Result<String, String> {
        let file =
            std::fs::File::open(file_path).map_err(|e| format!("Failed to open file: {}", e))?;
        let xplat = XPlatFile::new_native(file);
        self.upload_internal(parent_id, file_name, &xplat).await
    }
}
