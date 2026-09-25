pub mod download;
pub mod upload;

pub use download::*;
pub use upload::*;

use crate::net::manager::NetworkTransferManager;
use platrium_restapi::apis::configuration::Configuration;
use std::sync::Arc;

pub(crate) struct ApiInner {
    pub(crate) api_config: Arc<Configuration>,
    pub(crate) transfer_manager: Arc<NetworkTransferManager>,
}

#[cfg_attr(target_arch = "wasm32", wasm_bindgen::prelude::wasm_bindgen)]
#[derive(Clone, uniffi::Object)]
pub struct Api(Arc<ApiInner>);

impl Api {
    pub(crate) fn new(
        api_config: Arc<Configuration>,
        transfer_manager: Arc<NetworkTransferManager>,
    ) -> Self {
        Self(Arc::new(ApiInner {
            api_config,
            transfer_manager,
        }))
    }
}

#[cfg(not(target_arch = "wasm32"))]
#[uniffi::export(callback_interface)]
pub trait TransferEventListener: Send + Sync {
    fn on_event(&self, event: crate::net::transfers::NetTransferEvent);
}

#[cfg(not(target_arch = "wasm32"))]
#[uniffi::export]
impl Api {
    /// Subscribes to transfer events natively for Swift / Kotlin / C++.
    pub fn on_transfer_event(&self, listener: Box<dyn TransferEventListener>) {
        let mut rx = self.0.transfer_manager.subscribe_events();
        tokio::spawn(async move {
            while let Ok(event) = rx.recv().await {
                listener.on_event(event);
            }
        });
    }
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
pub struct TransferSubscription {
    cancel_token: tokio_util::sync::CancellationToken,
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
impl TransferSubscription {
    #[wasm_bindgen(js_name = unsubscribe)]
    pub fn unsubscribe(&self) {
        self.cancel_token.cancel();
    }
}

#[cfg(target_arch = "wasm32")]
#[wasm_bindgen::prelude::wasm_bindgen]
impl Api {
    /// Listens to transfer events specifically for files with cleanup handle.
    #[wasm_bindgen(js_name = onTransferEvent)]
    pub fn on_transfer_event(&self, callback: js_sys::Function) -> TransferSubscription {
        let mut rx = self.0.transfer_manager.subscribe_events();
        let cancel_token = tokio_util::sync::CancellationToken::new();
        let token_clone = cancel_token.clone();

        wasm_bindgen_futures::spawn_local(async move {
            loop {
                tokio::select! {
                    _ = token_clone.cancelled() => {
                        break;
                    }
                    res = rx.recv() => {
                        match res {
                            Ok(event) => {
                                if let Ok(js_val) = serde_wasm_bindgen::to_value(&event) {
                                    let _ = callback.call1(&js_sys::global(), &js_val);
                                }
                            }
                            Err(tokio::sync::broadcast::error::RecvError::Closed) => break,
                            Err(tokio::sync::broadcast::error::RecvError::Lagged(_)) => continue,
                        }
                    }
                }
            }
        });

        TransferSubscription { cancel_token }
    }
}
