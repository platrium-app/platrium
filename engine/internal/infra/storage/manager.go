package storage

import (
	"context"
	"fmt"
	"maps"
	"time"

	"platrium/internal/fsops"
	"platrium/pkg/syncx"
)

type Manager struct {
	supportedBackendTypes map[string]BackendFactory   // Maps a supported backend type string (like "s3") to its initialization logic
	activeBackends        *syncx.Map[string, Backend] // Stores the Initialized Backends
	cancels               *syncx.Map[string, context.CancelFunc]
	chunkValidationCh     chan fsops.ValidatedChunk
	chunkStore            *fsops.ChunkStore
}

func NewManager(chunkStore *fsops.ChunkStore) *Manager {
	return &Manager{
		supportedBackendTypes: make(map[string]BackendFactory),
		activeBackends:        syncx.NewMap[string, Backend](),
		cancels:               syncx.NewMap[string, context.CancelFunc](),
		chunkValidationCh:     make(chan fsops.ValidatedChunk, 10000),
		chunkStore:            chunkStore,
	}
}

// RegisterBackendType cleanly adds a compile-time supported storage engine to the platform.
func (m *Manager) RegisterBackendType(typeName string, backendInitFn BackendFactory) {
	m.supportedBackendTypes[typeName] = backendInitFn
}

// StartBackend instantiates a live running driver using a database configuration.
func (m *Manager) StartBackend(ctx context.Context, backendId string, cfg BackendConfig) error {
	backendInitFn, supported := m.supportedBackendTypes[cfg.Type]
	if !supported {
		return fmt.Errorf("Backend type %q is not supported!", cfg.Type)
	}

	// The manager runs the function pointer blindly. The driver handles its own configuration parsing.
	backend, err := backendInitFn(ctx, backendId, cfg.Config)
	if err != nil {
		return fmt.Errorf("Failed to initialize backend with ID %s: %w", backendId, err)
	}

	// Init Backend to Subscribe to Upload Events from Cloud / Local / etc
	backendCtx, cancel := context.WithCancel(ctx)
	backend.SubscribeUploadEvents(backendCtx, m.chunkValidationCh)

	m.activeBackends.Store(backendId, backend)
	m.cancels.Store(backendId, cancel)
	return nil
}

// GetActiveBackend retrieves an active StorageBackend instance by its unique backend ID.
// Primarily used by AttachedFS to fetch the backend instance for commit operations.
func (m *Manager) GetActiveBackend(backendId string) (Backend, bool) {
	return m.activeBackends.Load(backendId)
}

// GenerateChunkUploadURLs routes a set of missing chunk hashes to an active backend and returns presigned write targets.
func (m *Manager) GenerateChunkUploadURLs(ctx context.Context, chunkHashes []string) (map[string]string, error) {
	if len(chunkHashes) == 0 {
		return make(map[string]string), nil
	}

	// Route to the first available active storage backend instance (Future: RoundRobin/Placement/Capacity logic)
	var chosenBackend Backend
	m.activeBackends.Range(func(id string, backend Backend) bool {
		chosenBackend = backend
		return false // stop range on first active instance
	})

	if chosenBackend == nil {
		return nil, fmt.Errorf("no active storage backends registered")
	}

	chunks := make(map[string]ChunkUploadInfo, len(chunkHashes))
	for _, hash := range chunkHashes {
		chunks[hash] = ChunkUploadInfo{
			Path: GetShardedPath(ObjectTypeChunk, hash),
		}
	}

	return chosenBackend.GenerateChunkUploadURLs(ctx, chunks)
}

// GenerateChunkDownloadURLs routes a set of requested chunk hashes to their respective backends
// and returns presigned read targets, simultaneously updating their GC referenced timestamps.
func (m *Manager) GenerateChunkDownloadURLs(ctx context.Context, chunkHashes []string) (map[string]string, error) {
	if len(chunkHashes) == 0 {
		return make(map[string]string), nil
	}

	// 1. Fetch metadata in a purely read-only optimized point lookup
	current, err := m.chunkStore.GetChunks(ctx, chunkHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunk metadata: %w", err)
	}

	// 2. Group hashes by BackendID
	backendGroups := make(map[string][]string)
	for hash, meta := range current {
		if meta.State == fsops.ChunkStateValidated {
			backendGroups[meta.StorageBackend] = append(backendGroups[meta.StorageBackend], hash)
		}
	}

	// 3. Request presigned URLs from each specific backend
	results := make(map[string]string, len(chunkHashes))
	for backendId, hashes := range backendGroups {
		backend, ok := m.activeBackends.Load(backendId)
		if !ok {
			return nil, fmt.Errorf("storage backend %q is not active", backendId)
		}

		urls, err := backend.GenerateChunkDownloadURLs(ctx, hashes)
		if err != nil {
			return nil, fmt.Errorf("failed to generate download URLs for backend %q: %w", backendId, err)
		}

		maps.Copy(results, urls)
	}

	return results, nil
}

// StartChunkValidationWorker spins up a background worker that drains chunkValidationCh
// in batches of 100 chunks or every 250ms and marks them as PRESENT in the chunk metadata store.
func (m *Manager) StartChunkValidationWorker(ctx context.Context) {
	go func() {
		const batchSize = 100
		batch := make([]fsops.ValidatedChunk, 0, batchSize)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		// Pass the target context explicitly so we can swap it out during shutdown
		flush := func(flushCtx context.Context) {
			if len(batch) == 0 {
				return
			}

			_ = m.chunkStore.AddValidatedChunks(flushCtx, batch)
			batch = batch[:0]
		}

		for {
			select {
			case <-ctx.Done():
				// Create a fresh, independent context to guarantee the final database write completes
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				flush(shutdownCtx)
				cancel()
				return

			case hash, ok := <-m.chunkValidationCh:
				if !ok {
					// Channel closed by the app; flush remaining items using a fresh timeout context
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					flush(shutdownCtx)
					cancel()
					return
				}

				batch = append(batch, hash)
				if len(batch) >= batchSize {
					flush(ctx)
					ticker.Reset(250 * time.Millisecond) // Reset timer since we just cleared the queue
				}

			case <-ticker.C:
				flush(ctx)
			}
		}
	}()
}
