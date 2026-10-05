#[cfg(not(target_arch = "wasm32"))]
pub mod auth;
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

fn initialize_client_core(
    base_url: &str,
    bearer_token: Option<&str>,
) -> Result<(Arc<Configuration>, Arc<NetworkTransferManager>), crate::errors::PlatriumError> {
    let mut api_config = Configuration::new();
    api_config.base_path = base_url.to_string();

    if let Some(token) = bearer_token {
        // Sent on every request, including the raw chunk transfers that do not
        // go through the generated API functions.
        let mut value = reqwest::header::HeaderValue::from_str(&format!("Bearer {token}"))
            .map_err(|e| crate::errors::PlatriumError::InternalError(e.to_string()))?;
        value.set_sensitive(true);
        let mut headers = reqwest::header::HeaderMap::new();
        headers.insert(reqwest::header::AUTHORIZATION, value);
        api_config.client = platrium_restapi::apis::configuration::Client::builder()
            .default_headers(headers)
            .build()
            .map_err(|e| crate::errors::PlatriumError::InternalError(e.to_string()))?;
    }
    
    static MANAGER: std::sync::OnceLock<Arc<NetworkTransferManager>> = std::sync::OnceLock::new();
    let transfer_manager = MANAGER.get_or_init(|| Arc::new(NetworkTransferManager::new(5))).clone();
    
    Ok((Arc::new(api_config), transfer_manager))
}

#[cfg_attr(not(target_arch = "wasm32"), uniffi::export)]
#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
impl PlatriumClient {
    /// Creates a new Platrium SDK Client
    #[cfg_attr(not(target_arch = "wasm32"), uniffi::constructor)]
    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(constructor))]
    pub fn new(base_url: &str) -> Result<PlatriumClient, crate::errors::PlatriumError> {
        Self::build(base_url, None)
    }

    /// Access the Files API module
    #[cfg_attr(target_arch = "wasm32", wasm_bindgen(js_name = files))]
    pub fn files(&self) -> files::Api {
        self.make_files_api()
    }
}

impl PlatriumClient {
    fn build(base_url: &str, bearer_token: Option<&str>) -> Result<PlatriumClient, crate::errors::PlatriumError> {
        /* Initialize Cross Platform Logging */
        xplat::logging::init_xplat_logging();

        #[cfg(not(target_arch = "wasm32"))]
        let (api_config, transfer_manager) = xplat::runtime::get_runtime()
            .block_on(async { initialize_client_core(base_url, bearer_token) })?;

        #[cfg(target_arch = "wasm32")]
        let (api_config, transfer_manager) = initialize_client_core(base_url, bearer_token)?;

        Ok(Self(Arc::new(PlatriumClientInner {
            api_config,
            transfer_manager,
        })))
    }
}

/// Native-only constructors. Browsers authenticate with their session cookie,
/// so the WASM build has no token variant.
#[cfg(not(target_arch = "wasm32"))]
#[uniffi::export]
impl PlatriumClient {
    /// Creates a client that authenticates every request with a bearer token
    /// obtained from the sign-in flow (`/auth/authorize` then `/auth/token`).
    #[uniffi::constructor]
    pub fn with_token(base_url: &str, token: &str) -> Result<PlatriumClient, crate::errors::PlatriumError> {
        Self::build(base_url, Some(token))
    }
}

#[cfg(not(target_arch = "wasm32"))]
#[uniffi::export]
impl PlatriumClient {
    /// Access the authentication API: exchanging a sign-in code for a token
    /// and asking who the current token belongs to.
    pub fn auth(&self) -> auth::AuthApi {
        auth::AuthApi::new(self.0.api_config.clone())
    }
}
