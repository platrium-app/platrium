#[cfg(not(target_arch = "wasm32"))]
use std::sync::LazyLock;
#[cfg(not(target_arch = "wasm32"))]
use tokio::runtime::Runtime;

#[cfg(not(target_arch = "wasm32"))]
static RUNTIME: LazyLock<Runtime> = LazyLock::new(|| {
    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("Failed to initialize Tokio runtime for Platrium SDK")
});

#[cfg(not(target_arch = "wasm32"))]
pub fn get_runtime() -> &'static Runtime {
    &RUNTIME
}
