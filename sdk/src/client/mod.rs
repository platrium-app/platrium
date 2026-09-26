pub mod files;

use crate::net::manager::NetworkTransferManager;
use crate::xplat;
use platrium_restapi::apis::configuration::Configuration;
use std::sync::Arc;

struct PlatriumClientInner {
    #[allow(dead_code)]
    api_config: Arc<Configuration>,
    transfer_manager: Arc<NetworkTransferManager>,
}

/// The main entrypoint for the Platrium SDK
#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
#[derive(Clone, uniffi::Object)]
pub struct PlatriumClient(Arc<PlatriumClientInner>);

impl PlatriumClient {
    fn make_files_api(&self) -> files::Api {
        files::Api::new(
            self.0.api_config.clone(),
            self.0.transfer_manager.clone(),
        )
    }
}

#[cfg_attr(not(target_arch = "wasm32"), uniffi::export)]
#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
impl PlatriumClient {
    /// Creates a new Platrium SDK Client
    #[cfg_attr(not(target_arch = "wasm32"), uniffi::constructor)]
    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(constructor))]
    pub fn new(base_url: &str) -> Result<PlatriumClient, crate::errors::PlatriumError> {
        /* Initialize Cross Platform Logging */
        xplat::logging::init_xplat_logging();

        #[cfg(not(target_arch = "wasm32"))]
        let _guard = xplat::runtime::get_runtime().enter();

        let mut api_config = Configuration::new();
        api_config.base_path = base_url.to_string();
        let api_config = Arc::new(api_config);

        let transfer_manager = Arc::new(NetworkTransferManager::new(5));

        Ok(Self(Arc::new(PlatriumClientInner {
            api_config,
            transfer_manager,
        })))
    }

    /// Access the Files API module
    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(js_name = files))]
    pub fn files(&self) -> files::Api {
        self.make_files_api()
    }
}
