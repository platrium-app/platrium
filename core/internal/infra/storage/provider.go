package storage

import (
	"context"
	"path"
)

// StorageProvider acts as the orchestrator over a specific StorageBackend.
type StorageProvider struct {
	store AttachedFSStore
}

func NewStorageProvider(store AttachedFSStore) *StorageProvider {
	return &StorageProvider{
		store: store,
	}
}

// GenerateUploadURLs delegates to the correct backend.
func (s *StorageProvider) GenerateUploadURLs(ctx context.Context, chunkHashes []string) (map[string]string, error) {
	chunks := make(map[string]ChunkUploadInfo)
	for _, hash := range chunkHashes {
		chunks[hash] = ChunkUploadInfo{
			Path: GetShardedPath(ObjectTypeChunk, hash),
		}
	}

	// Hardcoded routing to attached FS backend for now
	backend := NewAttachedFSBackend(s.store)
	return backend.GenerateUploadURLs(ctx, "./data", chunks)
}

// GetShardedPath enforces 3-level prefix sharding to improve file system efficiency
// and faster performance for object-based stores. This is a unified layout for all providers.
func GetShardedPath(objectType ObjectType, hash string) string {
	return path.Join(
		string(objectType),
		hash[0:2],
		hash[2:4],
		hash[4:6],
		hash,
	)
}
