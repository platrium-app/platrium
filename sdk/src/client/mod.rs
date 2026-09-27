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

fn initialize_client_core(base_url: &str) -> (Arc<Configuration>, Arc<NetworkTransferManager>) {
    let mut api_config = Configuration::new();
    api_config.base_path = base_url.to_string();
    
    static MANAGER: std::sync::OnceLock<Arc<NetworkTransferManager>> = std::sync::OnceLock::new();
    let transfer_manager = MANAGER.get_or_init(|| Arc::new(NetworkTransferManager::new(5))).clone();
    
    (Arc::new(api_config), transfer_manager)
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
        let (api_config, transfer_manager) = xplat::runtime::get_runtime().block_on(async {
            initialize_client_core(base_url)
        });

        #[cfg(target_arch = "wasm32")]
        let (api_config, transfer_manager) = initialize_client_core(base_url);

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
