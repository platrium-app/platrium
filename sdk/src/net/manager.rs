use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::{RwLock, Semaphore, SemaphorePermit, broadcast};
use tokio_util::sync::CancellationToken;
use crate::net::transfers::*;

pub struct NetworkTransferManager {
    io_semaphore: Arc<Semaphore>,
    progress_tx: broadcast::Sender<TransferEvent>,
    active_transfers: RwLock<HashMap<String, TransferState>>,
}

impl NetworkTransferManager {
    pub fn new(concurrency_limit: usize) -> Self {
        let (progress_tx, _) = broadcast::channel(1024);
        Self {
            io_semaphore: Arc::new(Semaphore::new(concurrency_limit)),
            progress_tx,
            active_transfers: RwLock::new(HashMap::new()),
        }
    }

    /// Request a slot to perform a chunk transfer. This enforces the global concurrency limit.
    pub async fn acquire_slot(&self) -> SemaphorePermit<'_> {
        self.io_semaphore
            .acquire()
            .await
            .expect("Semaphore closed unexpectedly")
    }

    /// Register a new transfer with its total byte size and return its cancellation token.
    pub async fn register_transfer(&self, file_id: String, total_bytes: u64) -> CancellationToken {
        let token = CancellationToken::new();
        let state = TransferState {
            total_bytes,
            token: token.clone(),
        };

        let mut lock = self.active_transfers.write().await;
        lock.insert(file_id, state);

        token
    }

    /// Emits a progress event for a given file.
    pub async fn emit_progress(&self, file_id: &str, bytes_transferred: usize) {
        let lock = self.active_transfers.read().await;
        if let Some(state) = lock.get(file_id) {
            let _ = self.progress_tx.send(TransferEvent::Progress {
                file_id: file_id.to_string(),
                bytes_transferred: bytes_transferred as u64,
                total_bytes: state.total_bytes,
            });
        }
    }

    /// Emits completion
    pub async fn emit_completed(&self, file_id: &str) {
        let mut lock = self.active_transfers.write().await;
        lock.remove(file_id);
        let _ = self.progress_tx.send(TransferEvent::Completed {
            file_id: file_id.to_string(),
        });
    }

    /// Emits error
    pub async fn emit_error(&self, file_id: &str, error: String) {
        let mut lock = self.active_transfers.write().await;
        lock.remove(file_id);
        let _ = self.progress_tx.send(TransferEvent::Error {
            file_id: file_id.to_string(),
            error,
        });
    }

    /// Cancels a running transfer and cleans up its state.
    pub async fn cancel_transfer(&self, file_id: &str) {
        let mut lock = self.active_transfers.write().await;
        if let Some(state) = lock.remove(file_id) {
            state.token.cancel();
            let _ = self.progress_tx.send(TransferEvent::Cancelled {
                file_id: file_id.to_string(),
            });
        }
    }
}
