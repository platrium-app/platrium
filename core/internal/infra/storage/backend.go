package storage

import (
	"context"
	"encoding/json"
	"path"
	"platrium/internal/fsops"
)

// ObjectType defines the root classification for storage paths.
type ObjectType string

const (
	ObjectTypeChunk ObjectType = "chunk"
	// Future Object Type could have a combined category for Video Thumbnail, Profile Picture, etc.
	// that are stored as full files without CAS chunks.
)

// GetShardedPath enforces 3-level prefix sharding to improve file system efficiency
// and faster performance. This is a unified layout for all providers.
func GetShardedPath(objectType ObjectType, hash string) string {
	return path.Join(
		string(objectType),
		hash[0:2],
		hash[2:4],
		hash[4:6],
		hash,
	)
}

// ChunkUploadInfo contains metadata required by backends to generate precise upload URLs.
type ChunkUploadInfo struct {
	Path string
}

// BackendFactory is a factory function that enables registering a new backend type with StorageManager.
type BackendFactory func(ctx context.Context, backendId string, config json.RawMessage) (Backend, error)

type BackendConfig struct {
	Type   string          `json:"type"`   // "s3", "attached_fs"
	Config json.RawMessage `json:"config"` // Custom backend properties
}

// Backend represents a specific storage mechanism (AttachedFS, S3, Ceph, etc.).
type Backend interface {
	// GenerateChunkUploadURLs takes chunk metadata to build presigned write targets.
	GenerateChunkUploadURLs(ctx context.Context, chunks map[string]ChunkUploadInfo) (map[string]string, error)

	// GenerateChunkDownloadURLs builds direct-to-client presigned read targets to bypass the double download tax.
	GenerateChunkDownloadURLs(ctx context.Context, chunkHashes []string) (map[string]string, error)

	// SubscribeUploadEvents hooks into the backend's notification stream and funnels clean hashes to the app layer.
	// Used to add VALIDATED chunks to the store.
	SubscribeUploadEvents(ctx context.Context, chunkValidationCh chan<- fsops.ValidatedChunk)
}
