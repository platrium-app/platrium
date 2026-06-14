package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"platrium/internal/providers"
)

// ObjectsHandler manages the REST endpoints for object/chunk generation and uploads.
type ObjectsHandler struct {
	storageProvider *providers.StorageProvider
}

// NewObjectsHandler initializes a new ObjectsHandler with its required dependencies.
func NewObjectsHandler(storageProvider *providers.StorageProvider) *ObjectsHandler {
	return &ObjectsHandler{
		storageProvider: storageProvider,
	}
}

// Routes returns a chi.Router with all object-related endpoints mounted.
func (h *ObjectsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/upload", h.RequestUploadHandler)
	return r
}

// UploadRequest defines the JSON payload for requesting upload URLs.
type UploadRequest struct {
	Hashes []string `json:"hashes"`
}

// RequestUploadHandler handles POST /api/objects/upload, generating presigned upload URLs for the given chunk hashes.
//
// @Summary      Request Upload URLs
// @Description  Generates presigned upload URLs for a given set of object hashes.
// @Tags         objects
// @Accept       json
// @Produce      json
// @Param        request body UploadRequest true "List of SHA256 hashes"
// @Success      200  {object}  map[string]string
// @Failure      400  {string}  string "Bad Request"
// @Failure      500  {string}  string "Internal Server Error"
// @Router       /api/objects/upload [post]
func (h *ObjectsHandler) RequestUploadHandler(w http.ResponseWriter, r *http.Request) {
	var req UploadRequest
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
