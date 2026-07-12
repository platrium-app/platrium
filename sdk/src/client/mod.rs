pub mod files;

use platrium_restapi::apis::configuration::Configuration;
use std::sync::Arc;

/// The main entrypoint for the Platrium SDK
#[derive(Clone)]
pub struct PlatriumClient {
    pub(crate) api_config: Arc<Configuration>,
    
    /// File operations client
    pub files: files::FilesClient,
}

impl PlatriumClient {
    /// Creates a new Platrium SDK Client
    pub fn new(base_url: &str) -> Result<Self, String> {
        let mut api_config = Configuration::new();
        api_config.base_path = base_url.to_string();
        let api_config = Arc::new(api_config);

        Ok(Self {
            files: files::FilesClient::new(api_config.clone()),
            api_config,
        })
    }
}
