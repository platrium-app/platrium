package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"platrium/internal/infra/storage"
	"platrium/pkg/constants"
)

// TODO: This needs moving. API folder needs to be removed.
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
	r.Get("/{backendId}/{hash}", h.AttachedFSDownloadHandler)
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

// AttachedFSDownloadHandler handles GET /api/attachedfs/{backendId}/{hash}, streaming chunk data from local storage.
//
// @Summary      Stream Object Chunk
// @Description  Directly streams a chunk payload from local storage using a signed download URL. Supports HTTP Range requests natively.
// @Tags         attachedfs
// @Produce      application/octet-stream
// @Param        backendId path string true "The storage backend ID"
// @Param        hash path string true "The chunk hash"
// @Success      200  {file}  file "Chunk data"
// @Failure      400  {string}  string "Bad Request"
// @Failure      401  {string}  string "Unauthorized - Expired or Invalid signature"
// @Router       /api/attachedfs/{backendId}/{hash} [get]
func (h *AttachedFSHandler) AttachedFSDownloadHandler(w http.ResponseWriter, r *http.Request) {
	backendId := chi.URLParam(r, "backendId")
	hash := chi.URLParam(r, "hash")
	sig := r.URL.Query().Get("sig")
	expiresStr := r.URL.Query().Get("expires")

	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil || time.Now().Unix() > expires {
		http.Error(w, "URL expired or invalid expiration format", http.StatusUnauthorized)
		return
	}

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

	if !afsBackend.VerifyReadURL(hash, expiresStr, sig) {
		http.Error(w, "Invalid read signature", http.StatusUnauthorized)
		return
	}

	stream, err := afsBackend.GetChunkReader(hash)
	if err != nil {
		http.Error(w, "Chunk not found", http.StatusNotFound)
		return
	}
	defer stream.Close()

	// Prevent ServeContent from sniffing the MIME type, since chunks are raw binary blobs.
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, hash, time.Time{}, stream)
}
