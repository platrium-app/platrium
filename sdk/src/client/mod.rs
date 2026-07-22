pub mod files;

use crate::net::manager::NetworkTransferManager;
use crate::xplat;
use platrium_restapi::apis::configuration::Configuration;
use std::sync::Arc;

/// The main entrypoint for the Platrium SDK
#[derive(Clone, uniffi::Object)]
pub struct PlatriumClient {
    #[allow(dead_code)]
    pub(crate) api_config: Arc<Configuration>,
    pub(crate) transfer_manager: Arc<NetworkTransferManager>,
}

#[uniffi::export]
impl PlatriumClient {
    /// Creates a new Platrium SDK Client
    #[uniffi::constructor]
    pub fn new(base_url: &str) -> Result<Self, crate::errors::PlatriumError> {
        /* Initialize Cross Platform Logging */
        xplat::logging::init_xplat_logging();

        let mut api_config = Configuration::new();
        api_config.base_path = base_url.to_string();
        let api_config = Arc::new(api_config);

        let transfer_manager = Arc::new(NetworkTransferManager::new(5));

        Ok(Self {
            api_config,
            transfer_manager,
        })
    }

    /// Access the Files API module
    pub fn files(&self) -> Arc<files::Api> {
        Arc::new(files::Api::new(
            self.api_config.clone(),
            self.transfer_manager.clone(),
        ))
    }
}
