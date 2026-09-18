use crate::net::transfers::*;
use std::collections::HashMap;
use std::sync::Arc;
use std::sync::atomic::AtomicU64;
use tokio::sync::{RwLock, Semaphore, SemaphorePermit, broadcast};
use tokio_util::sync::CancellationToken;

struct TransferState {
    pub(crate) direction: TransferDirection,
    pub(crate) status: RwLock<TransferStatus>,
    pub(crate) bytes_transferred: Arc<AtomicU64>,
    pub(crate) total_bytes: u64,
    pub(crate) metadata: TransferMetadata,
    pub(crate) token: CancellationToken,
}

pub(crate) struct NetworkTransferManager {
    io_semaphore: Arc<Semaphore>,
    transfer_events: broadcast::Sender<NetTransferEvent>,
    active_transfers: RwLock<HashMap<String, TransferState>>,
}

const TAG: &str = "NetworkTransferManager";
impl NetworkTransferManager {
    pub(crate) fn new(concurrency_limit: usize) -> Self {
        let (transfer_events, _) = broadcast::channel(1024);
        Self {
            io_semaphore: Arc::new(Semaphore::new(concurrency_limit)),
            transfer_events,
            active_transfers: RwLock::new(HashMap::new()),
        }
    }

    /// Request a slot to perform a chunk transfer. This enforces the global concurrency limit.
    pub(crate) async fn acquire_slot(&self) -> SemaphorePermit<'_> {
        self.io_semaphore
            .acquire()
            .await
            .expect("Semaphore closed unexpectedly")
    }

    pub(crate) fn subscribe_events(&self) -> broadcast::Receiver<NetTransferEvent> {
        self.transfer_events.subscribe()
    }

    /// Initializes a new transfer (Preparing state)
    pub(crate) async fn init_transfer(
        &self,
        transfer_id: &str,
        direction: TransferDirection,
        total_bytes: u64,
        metadata: TransferMetadata,
    ) -> CancellationToken {
        let token = CancellationToken::new();
        let state = TransferState {
            direction: direction.clone(),
            status: RwLock::new(TransferStatus::Preparing),
            bytes_transferred: Arc::new(std::sync::atomic::AtomicU64::new(0)),
            total_bytes,
            metadata: metadata.clone(),
            token: token.clone(),
        };

        let mut lock = self.active_transfers.write().await;
        log::debug!(target: TAG, "Init Transfer: ID {} ({} bytes)", transfer_id, total_bytes);
        lock.insert(transfer_id.to_string(), state);

        let _ = self.transfer_events.send(NetTransferEvent {
            transfer_id: transfer_id.to_string(),
            direction,
            status: TransferStatus::Preparing,
            bytes_transferred: 0,
            total_bytes,
            metadata,
        });

        token
    }

    /// Transitions a transfer to the Transferring state
    pub(crate) async fn start_transfer(&self, transfer_id: &str) {
        let lock = self.active_transfers.read().await;
        if let Some(state) = lock.get(transfer_id) {
            let mut status_lock = state.status.write().await;
            *status_lock = TransferStatus::Transferring;

            let bytes = state
                .bytes_transferred
                .load(std::sync::atomic::Ordering::Relaxed);
            let _ = self.transfer_events.send(NetTransferEvent {
                transfer_id: transfer_id.to_string(),
                direction: state.direction.clone(),
                status: TransferStatus::Transferring,
                bytes_transferred: bytes,
                total_bytes: state.total_bytes,
                metadata: state.metadata.clone(),
            });
        }
    }

    /// Atomically increments the transferred bytes and emits a progress event.
    pub(crate) async fn add_transferred_bytes(&self, transfer_id: &str, bytes: u64) {
        let lock = self.active_transfers.read().await;
        if let Some(state) = lock.get(transfer_id) {
            let new_bytes = state
                .bytes_transferred
                .fetch_add(bytes, std::sync::atomic::Ordering::Relaxed)
                + bytes;

            log::debug!(target: TAG, "Transfer Progress: ID {} ({}/{} bytes)", transfer_id, new_bytes, state.total_bytes);

            let _ = self.transfer_events.send(NetTransferEvent {
                transfer_id: transfer_id.to_string(),
                direction: state.direction.clone(),
                status: TransferStatus::Transferring,
                bytes_transferred: new_bytes,
                total_bytes: state.total_bytes,
                metadata: state.metadata.clone(),
            });
        }
    }

    /// Emits completion
    pub(crate) async fn complete_transfer(&self, transfer_id: &str) {
        let mut lock = self.active_transfers.write().await;
        if let Some(state) = lock.remove(transfer_id) {
            let bytes = state
                .bytes_transferred
                .load(std::sync::atomic::Ordering::Relaxed);
            let _ = self.transfer_events.send(NetTransferEvent {
                transfer_id: transfer_id.to_string(),
                direction: state.direction.clone(),
                status: TransferStatus::Completed,
                bytes_transferred: bytes,
                total_bytes: state.total_bytes,
                metadata: state.metadata.clone(),
            });
        }
    }

    /// Emits error
    pub(crate) async fn emit_error(&self, transfer_id: &str, error: String) {
        let mut lock = self.active_transfers.write().await;
        if let Some(state) = lock.remove(transfer_id) {
            let bytes = state
                .bytes_transferred
                .load(std::sync::atomic::Ordering::Relaxed);
            let _ = self.transfer_events.send(NetTransferEvent {
                transfer_id: transfer_id.to_string(),
                direction: state.direction.clone(),
                status: TransferStatus::Error { message: error },
                bytes_transferred: bytes,
                total_bytes: state.total_bytes,
                metadata: state.metadata.clone(),
            });
        }
    }

    /// Cancels a running transfer and cleans up its state.
    pub(crate) async fn cancel_transfer(&self, transfer_id: &str) {
        let mut lock = self.active_transfers.write().await;
        if let Some(state) = lock.remove(transfer_id) {
            state.token.cancel();
            let bytes = state
                .bytes_transferred
                .load(std::sync::atomic::Ordering::Relaxed);
            let _ = self.transfer_events.send(NetTransferEvent {
                transfer_id: transfer_id.to_string(),
                direction: state.direction.clone(),
                status: TransferStatus::Cancelled,
                bytes_transferred: bytes,
                total_bytes: state.total_bytes,
                metadata: state.metadata.clone(),
            });
        }
    }
}
