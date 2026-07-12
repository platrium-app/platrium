#[cfg(not(target_arch = "wasm32"))]
use std::fs::File;

#[cfg(target_arch = "wasm32")]
use web_sys::File as WebFile;

pub enum XPlatFile {
    #[cfg(not(target_arch = "wasm32"))]
    Native(File),

    #[cfg(target_arch = "wasm32")]
    Wasm(WebFile),
}

impl XPlatFile {
    #[cfg(not(target_arch = "wasm32"))]
    pub fn new_native(file: File) -> Self {
        XPlatFile::Native(file)
    }

    #[cfg(target_arch = "wasm32")]
    pub fn new_wasm(file: WebFile) -> Self {
        XPlatFile::Wasm(file)
    }

    pub fn size(&self) -> u64 {
        match self {
            #[cfg(not(target_arch = "wasm32"))]
            XPlatFile::Native(file) => file.metadata().map(|m| m.len()).unwrap_or(0),
            #[cfg(target_arch = "wasm32")]
            XPlatFile::Wasm(file) => file.size() as u64,
        }
    }

    pub async fn read_exact_at(&self, offset: u64, size: usize) -> Result<Vec<u8>, String> {
        match self {
            #[cfg(not(target_arch = "wasm32"))]
            XPlatFile::Native(file) => {
                use std::io::{Read, Seek, SeekFrom};
                let mut f = file.try_clone().map_err(|e| e.to_string())?;
                f.seek(SeekFrom::Start(offset)).map_err(|e| e.to_string())?;
                let mut buffer = vec![0; size];
                f.read_exact(&mut buffer).map_err(|e| e.to_string())?;
                Ok(buffer)
            }
            #[cfg(target_arch = "wasm32")]
            XPlatFile::Wasm(file) => {
                use wasm_bindgen::JsCast;
                let blob = file
                    .slice_with_f64_and_f64(offset as f64, (offset + size as u64) as f64)
                    .map_err(|_| "Failed to slice blob".to_string())?;

                let promise = blob.array_buffer();
                let future = wasm_bindgen_futures::JsFuture::from(promise);
                let js_val = future
                    .await
                    .map_err(|_| "Failed to read array buffer".to_string())?;

                let array_buffer = js_val.unchecked_into::<js_sys::ArrayBuffer>();
                let uint8_array = js_sys::Uint8Array::new(&array_buffer);
                let mut buffer = vec![0; size];
                uint8_array.copy_to(&mut buffer);

                Ok(buffer)
            }
        }
    }
}
