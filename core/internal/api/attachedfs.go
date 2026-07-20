package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"platrium/internal/infra/storage"
	"platrium/pkg/constants"
)

// AttachedFSHandler manages direct stream uploads for the local attached file system backend.
type AttachedFSHandler struct {
	storageManager *storage.Manager
}

// NewAttachedFSHandler initializes a new AttachedFSHandler with the attached file system backend.
func NewAttachedFSHandler(storageManager *storage.Manager) *AttachedFSHandler {
	return &AttachedFSHandler{
		storageManager: storageManager,
	}
}

// Routes returns a chi.Router with the local file upload endpoints mounted.
func (h *AttachedFSHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Put("/{backendId}/{writeId}", h.AttachedFSUploadHandler)
	return r
}

// AttachedFSUploadHandler handles PUT /api/attachedfs/{writeId}, streaming the body directly to the storage backend.
//
// @Summary      Stream Object Chunk
// @Description  Directly streams a chunk payload into local storage using a temporary writeId. Performs real-time SHA256 cryptographic verification.
// @Tags         attachedfs
// @Accept       application/octet-stream
// @Produce      json
// @Param        writeId path string true "The temporary write session UUID"
// @Param        file body string true "The binary chunk data"
// @Success      200  {string}  string "OK"
// @Failure      400  {string}  string "Bad Request - Invalid Hash or Stream"
// @Failure      401  {string}  string "Unauthorized - Expired or Invalid writeId"
// @Failure      500  {string}  string "Internal Server Error"
// @Router       /api/attachedfs/{writeId} [put]
func (h *AttachedFSHandler) AttachedFSUploadHandler(w http.ResponseWriter, r *http.Request) {
	backendId := chi.URLParam(r, "backendId")
	writeSig := r.URL.Query().Get("sig")
	writeId := chi.URLParam(r, "writeId")

	// Enforce strict chunk size limits at the HTTP stream level
	r.Body = http.MaxBytesReader(w, r.Body, constants.DedupChunkSizeBytes)
	defer r.Body.Close()

	// Get the Backend from Storage Manager
	backend, exists := h.storageManager.GetActiveBackend(backendId)
	if !exists {
		http.Error(w, "Invalid Storage Manager Backend ID", http.StatusBadRequest)
		return
	}

	afsBackend, ok := backend.(*storage.AttachedFSBackend)
	if !ok {
		http.Error(w, "Backend is not an AttachedFS instance", http.StatusBadRequest)
		return
	}

	if err := afsBackend.CommitLocalWrite(r.Context(), writeId, writeSig, r.Body); err != nil {
		log.Printf("upload failed for writeId %s: %v", writeId, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
