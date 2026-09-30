package objects

import (
	"platrium/internal/infra/storage"

	"github.com/go-chi/chi/v5"
)

// NewRouter aggregates all object-specific handlers into a single router.
func NewRouter(storageProvider *storage.Manager) chi.Router {
	r := chi.NewRouter()

	chunkHandler := NewChunkHandler(storageProvider)

	r.Post("/presign", chunkHandler.Presign)

	return r
}
