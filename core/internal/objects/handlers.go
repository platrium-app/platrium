package objects

import (
	"encoding/json"
	"net/http"
	
	"platrium/internal/infra/storage"
)

// ChunkHandler manages raw binary blobs (chunks) in the underlying object storage.
// NOTE: Future features like Object Garbage Collection, chunk deduplication analytics,
// or raw chunk retrieval should go in this domain.
type ChunkHandler struct {
	storageProvider *storage.StorageProvider
}

func NewChunkHandler(sp *storage.StorageProvider) *ChunkHandler {
	return &ChunkHandler{storageProvider: sp}
}

type PresignRequest struct {
	Hashes []string `json:"hashes"`
}

// Presign generates upload URLs (S3 presigned or AttachedFS local URLs) for a given set of object hashes.
func (h *ChunkHandler) Presign(w http.ResponseWriter, r *http.Request) {
	var req PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	urls, err := h.storageProvider.GenerateUploadURLs(r.Context(), req.Hashes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(urls)
}
