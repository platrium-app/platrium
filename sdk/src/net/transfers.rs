use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Serialize, Deserialize, uniffi::Enum)]
#[cfg_attr(target_arch = "wasm32", derive(tsify::Tsify))]
#[cfg_attr(target_arch = "wasm32", tsify(into_wasm_abi, from_wasm_abi))]
pub enum TransferDirection {
    Upload,
    Download,
}

#[derive(Clone, Debug, Serialize, Deserialize, uniffi::Enum)]
#[serde(tag = "type")]
#[cfg_attr(target_arch = "wasm32", derive(tsify::Tsify))]
#[cfg_attr(target_arch = "wasm32", tsify(into_wasm_abi, from_wasm_abi))]
pub enum TransferMetadata {
    FileChunk {
        folder_id: String,
        file_name: String,
    },
    // Example of Future Transfer Type:
    // StaticAsset {
    //     asset_id: String,
    // },
}

#[derive(Clone, Debug, Serialize, Deserialize, uniffi::Enum)]
#[serde(tag = "type")]
#[cfg_attr(target_arch = "wasm32", derive(tsify::Tsify))]
#[cfg_attr(target_arch = "wasm32", tsify(into_wasm_abi, from_wasm_abi))]
pub enum TransferStatus {
    Preparing,
    Transferring,
    Completed,
    Error { message: String },
    Cancelled,
}

#[derive(Clone, Debug, Serialize, Deserialize, uniffi::Record)]
#[serde(rename_all = "camelCase")]
#[cfg_attr(target_arch = "wasm32", derive(tsify::Tsify))]
#[cfg_attr(target_arch = "wasm32", tsify(into_wasm_abi, from_wasm_abi))]
pub struct NetTransferEvent {
    pub transfer_id: String,
    pub direction: TransferDirection,
    pub status: TransferStatus,
    pub bytes_transferred: u64,
    pub total_bytes: u64,
    pub metadata: TransferMetadata,
}

