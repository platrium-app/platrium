use tokio_util::sync::CancellationToken;

#[derive(Clone, Debug, uniffi::Enum)]
pub enum TransferEvent {
    Progress {
        file_id: String,
        bytes_transferred: u64,
        total_bytes: u64,
    },
    Completed {
        file_id: String,
    },
    Error {
        file_id: String,
        error: String,
    },
    Cancelled {
        file_id: String,
    },
}

pub struct TransferState {
    pub total_bytes: u64,
    pub token: CancellationToken,
}
