#[cfg(not(target_arch = "wasm32"))]
use std::fs::File;

#[cfg(target_arch = "wasm32")]
use web_sys::File as WebFile;

#[cfg(not(target_arch = "wasm32"))]
use std::sync::Arc;

#[cfg(target_arch = "wasm32")]
#[derive(Clone)]
pub enum BrowserFile {
    WebFile(WebFile),
    WriteStream(web_sys::WritableStream),
}

#[derive(Clone)]
pub enum XPlatFile {
    #[cfg(not(target_arch = "wasm32"))]
    Native(Arc<File>),

    #[cfg(target_arch = "wasm32")]
    Wasm(BrowserFile),
}

impl XPlatFile {
    #[cfg(not(target_arch = "wasm32"))]
    pub fn new(file: File) -> Self {
        XPlatFile::Native(Arc::new(file))
    }

    #[cfg(target_arch = "wasm32")]
    pub fn new(file: WebFile) -> Self {
        XPlatFile::Wasm(BrowserFile::WebFile(file))
    }

    #[cfg(target_arch = "wasm32")]
    pub fn new_write_stream(stream: web_sys::WritableStream) -> Self {
        XPlatFile::Wasm(BrowserFile::WriteStream(stream))
    }

    pub fn size(&self) -> u64 {
        match self {
            #[cfg(not(target_arch = "wasm32"))]
            XPlatFile::Native(file) => file.metadata().map(|m| m.len()).unwrap_or(0),
            #[cfg(target_arch = "wasm32")]
            XPlatFile::Wasm(browser_file) => match browser_file {
                BrowserFile::WebFile(file) => file.size() as u64,
                BrowserFile::WriteStream(_) => 0,
            },
        }
    }

    pub async fn read_exact_at(&self, offset: u64, size: usize) -> Result<Vec<u8>, String> {
        match self {
            #[cfg(not(target_arch = "wasm32"))]
            XPlatFile::Native(file) => {
                use std::os::unix::fs::FileExt;
                let mut buffer: Vec<u8> = vec![0; size];
                file.read_exact_at(&mut buffer, offset)
                    .map_err(|e| e.to_string())?;

                Ok(buffer)
            }

            #[cfg(target_arch = "wasm32")]
            XPlatFile::Wasm(browser_file) => match browser_file {
                BrowserFile::WriteStream(_) => Err("Cannot read from a Write Stream".to_string()),
                BrowserFile::WebFile(file) => {
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
            },
        }
    }

    #[cfg(not(target_arch = "wasm32"))]
    pub async fn write_exact_at(&self, offset: u64, bytes: &[u8]) -> Result<(), String> {
        match self {
            XPlatFile::Native(file) => {
                use std::os::unix::fs::FileExt;
                file.write_all_at(bytes, offset).map_err(|e| e.to_string())
            }
        }
    }

    #[cfg(target_arch = "wasm32")]
    pub async fn write_sequentially(&self, bytes: &[u8]) -> Result<(), String> {
        match self {
            XPlatFile::Wasm(browser_file) => match browser_file {
                BrowserFile::WebFile(_) => {
                    Err("Cannot write to a read-only WebFile. Use a Stream.".to_string())
                }

                BrowserFile::WriteStream(stream) => {
                    let writer = stream
                        .get_writer()
                        .map_err(|_| "Failed to get WritableStreamDefaultWriter".to_string())?;

                    let uint8_array = js_sys::Uint8Array::new_with_length(bytes.len() as u32);
                    uint8_array.copy_from(bytes);

                    let promise = writer.write_with_chunk(&uint8_array);
                    wasm_bindgen_futures::JsFuture::from(promise)
                        .await
                        .map_err(|_| "JS stream write promise rejected".to_string())?;

                    writer.release_lock();
                    Ok(())
                }
            },
        }
    }

    pub async fn get_mime_type(&self) -> String {
        let size = std::cmp::min(self.size(), 4096) as usize;
        self.read_exact_at(0, size)
            .await
            .ok()
            .and_then(|buffer| infer::get(&buffer).map(|k| k.mime_type().to_string()))
            .unwrap_or_else(|| "application/octet-stream".to_string())
    }
}
