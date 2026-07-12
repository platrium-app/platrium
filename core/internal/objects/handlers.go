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

// Presign godoc
// @Summary      Generate upload URLs for chunks
// @Description  Generates presigned URLs for uploading raw binary blobs (chunks) directly to the storage backend.
// @Tags         Objects
// @Accept       json
// @Produce      json
// @Param        request body PresignRequest true "Chunk hashes to presign"
// @Success      200  {object}  map[string]string "Returns map of hash to upload URL"
// @Failure      400  {string}  string "Invalid JSON"
// @Failure      500  {string}  string "Internal Server Error"
// @Router       /objects/presign [post]
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
