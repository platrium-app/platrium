package objects

import (
	"github.com/go-chi/chi/v5"
	"platrium/internal/infra/storage"
)

// NewRouter aggregates all object-specific handlers into a single router.
func NewRouter(storageProvider *storage.StorageProvider) chi.Router {
	r := chi.NewRouter()

	chunkHandler := NewChunkHandler(storageProvider)

	r.Post("/presign", chunkHandler.Presign)
	
	return r
}
