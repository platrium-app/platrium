package fsops

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"platrium/internal/infra/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// HTTPHandler manages the REST endpoints for file system operations.
type HTTPHandler struct {
	fsOps           *FSOps
	manifestRepo    *ManifestRepo
	storageProvider *storage.StorageProvider
}

// NewHTTPHandler initializes a new HTTPHandler with its required dependencies.
func NewHTTPHandler(fsOps *FSOps, manifestRepo *ManifestRepo, storageProvider *storage.StorageProvider) *HTTPHandler {
	return &HTTPHandler{
		fsOps:           fsOps,
		manifestRepo:    manifestRepo,
		storageProvider: storageProvider,
	}
}

// Routes returns a chi.Router with all fs-related endpoints mounted.
func (h *HTTPHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/upload", h.RequestUploadHandler)
	r.Post("/createfile", h.CreateFileHandler)
	return r
}

// UploadRequest defines the JSON payload for requesting upload URLs.
type UploadRequest struct {
	Hashes []string `json:"hashes"`
}

// RequestUploadHandler generates presigned upload URLs for a given set of object hashes.
func (h *HTTPHandler) RequestUploadHandler(w http.ResponseWriter, r *http.Request) {
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

type CreateFileRequest struct {
	TenantID string   `json:"tenant_id"` // TODO: Rip from JWT middleware later
	ParentID string   `json:"parent_id"`
	FileName string   `json:"file_name"`
	Hashes   []string `json:"hashes"`
}

// CreateFileHandler executes the Hybrid Manifest File Creation flow.
func (h *HTTPHandler) CreateFileHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: File Alr Exists, etc.

	// 1. Decode hex hashes into binary to save 50% storage overhead in KVStore
	var binaryHashes [][]byte
	for _, hexHash := range req.Hashes {
		bin, err := hex.DecodeString(hexHash)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid hex hash %s: %v", hexHash, err), http.StatusBadRequest)
			return
		}
		binaryHashes = append(binaryHashes, bin)
	}

	// 2. Generate new File ID & Version
	fileId := uuid.New().String()
	version := 1 // Initial creation is always v1

	// 3. Save Paged Binary Manifest to KVStore
	if err := h.manifestRepo.SaveManifest(r.Context(), fileId, version, binaryHashes); err != nil {
		http.Error(w, fmt.Sprintf("failed to save manifest: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Hybrid Graph Optimization: Inline chunks if small enough
	var inlineChunks []string
	if len(req.Hashes) <= 4 {
		// Store as native string array in Neo4j for instant frontend access
		inlineChunks = req.Hashes
	}

	manifestPath := fmt.Sprintf("%s:%s:v%d", NamespaceManifest, fileId, version)

	// 5. Execute Graph Transaction (MERGE idempotent creation)
	if err := h.fsOps.CreateFile(r.Context(), req.TenantID, req.ParentID, fileId, req.FileName, manifestPath, inlineChunks); err != nil {
		http.Error(w, fmt.Sprintf("failed to create file node: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"file_id": fileId})
}
