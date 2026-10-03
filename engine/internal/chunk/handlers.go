package chunk

import (
	"platrium/internal/infra/storage"
)

// TODO: (9/29) OBSOLETE -- Maybe move chunk.go from fsops to here for future GC
// and to revive?

// ChunkHandler manages raw binary blobs (chunks) in the underlying object storage.
// NOTE: Future features like Object Garbage Collection, chunk deduplication analytics,
// or raw chunk retrieval should go in this domain.
type ChunkHandler struct {
	storageProvider *storage.Manager
}

func NewChunkHandler(sp *storage.Manager) *ChunkHandler {
	return &ChunkHandler{storageProvider: sp}
}
