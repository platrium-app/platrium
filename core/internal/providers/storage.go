package providers

import (
	"context"
	"path"

	"platrium/internal/adapters/storage"
	"platrium/internal/repositories"
)

// StorageProvider acts as the orchestrator over a specific StorageBackend.
type StorageProvider struct {
	writesRepo repositories.AttachedFSWritesRepository
}

func NewStorageProvider(writesRepo repositories.AttachedFSWritesRepository) *StorageProvider {
	return &StorageProvider{
		writesRepo: writesRepo,
	}
}

// GenerateUploadURLs delegates to the correct backend.
func (s *StorageProvider) GenerateUploadURLs(ctx context.Context, chunkHashes []string) (map[string]string, error) {
	chunks := make(map[string]storage.ChunkUploadInfo)
	for _, hash := range chunkHashes {
		chunks[hash] = storage.ChunkUploadInfo{
			Path: GetShardedPath(storage.ObjectTypeChunk, hash),
			Size: 0, // Size is unknown at this point, but structurally supported
		}
	}

	// Hardcoded routing to attached FS backend for now
	backend := storage.NewAttachedFSBackend(s.writesRepo)
	return backend.GenerateUploadURLs(ctx, "./data", chunks)
}

// GetShardedPath enforces 3-level prefix sharding to improve file system efficiency
// and faster performance for object-based stores. This is a unified layout for all providers.
func GetShardedPath(objectType storage.ObjectType, hash string) string {
	return path.Join(
		string(objectType),
		hash[0:2],
		hash[2:4],
		hash[4:6],
		hash,
	)
}
