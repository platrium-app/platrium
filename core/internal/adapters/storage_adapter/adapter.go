package storage_adapter

import (
	"context"
)

// TODO: rename to adapter
// ObjectType defines the root classification for storage paths.
type ObjectType string

const (
	ObjectTypeChunk ObjectType = "chunk"
)

// ChunkUploadInfo contains metadata required by backends to generate precise upload URLs.
type ChunkUploadInfo struct {
	Path string
}

// StorageBackend represents a specific storage mechanism (Local, S3, etc.).
type StorageBackend interface {
	// GenerateUploadURLs takes a map of Hash -> ChunkUploadInfo and returns a map of Hash -> Presigned URL.
	GenerateUploadURLs(ctx context.Context, location string, chunks map[string]ChunkUploadInfo) (map[string]string, error)
}
