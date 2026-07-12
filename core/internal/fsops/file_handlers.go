package fsops

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"platrium/internal/infra/storage"
)

type CreateFileRequest struct {
	ParentID string   `json:"parent_id"`
	FileName string   `json:"file_name"`
	Hashes   []string `json:"hashes"`
}

type CheckMissingChunksRequest struct {
	Hashes []string `json:"hashes"`
}

type FileHandler struct {
	fsOps           *FSOps
	storageProvider *storage.StorageProvider
}

func NewFileHandler(fsOps *FSOps, storageProvider *storage.StorageProvider) *FileHandler {
	return &FileHandler{
		fsOps:           fsOps,
		storageProvider: storageProvider,
	}
}

// CreateFile godoc
// @Summary      Create a new file
// @Description  Creates a new file in the graph database. Hashes provided will be processed inline if <= 4, otherwise paged to KV store.
// @Tags         FileSystem
// @Accept       json
// @Produce      json
// @Param        request body CreateFileRequest true "File creation parameters"
// @Success      201  {object}  map[string]string "Returns file_id"
// @Failure      400  {string}  string "Invalid JSON"
// @Failure      500  {string}  string "Internal Server Error"
// @Router       /fs/createfile [post]
func (h *FileHandler) CreateFile(w http.ResponseWriter, r *http.Request) {
	var req CreateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: Hardcoded tenant ID for now, replace with JWT middleware later
	tenantId := "10ea4b03-d6dd-40bc-8b29-12b02efac041"

	fileId, err := h.fsOps.CreateFile(r.Context(), tenantId, req.ParentID, req.FileName, req.Hashes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"file_id": fileId})
}

func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	fileId := chi.URLParam(r, "fileId")
	tenantId := r.URL.Query().Get("tenantId") // TODO: Rip from JWT middleware later

	if fileId == "" || tenantId == "" {
		http.Error(w, "fileId and tenantId are required", http.StatusBadRequest)
		return
	}

	// TODO: Support range requests (e.g., NSFileProviders video playback)

	file, err := h.fsOps.GetFile(r.Context(), tenantId, fileId)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	if file.ManifestPath != "" {
		// TODO: Implement large manifest paging downloads
		http.Error(w, "Downloading massive files via manifests is not yet implemented", http.StatusNotImplemented)
		return
	}

	// Stream inline chunks directly without buffering to memory
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+file.Name+`"`)

	for _, chunkHash := range file.InlineChunks {
		chunkReader, err := h.storageProvider.GetChunk(r.Context(), chunkHash)
		if err != nil {
			// If we fail mid-stream, we cannot return an HTTP 500 because the 200 OK
			// and headers have already been sent. We MUST forcefully abort the TCP
			// connection so the client knows the download is incomplete/corrupted.
			panic(http.ErrAbortHandler)
		}

		if _, err := io.Copy(w, chunkReader); err != nil {
			chunkReader.Close()
			panic(http.ErrAbortHandler)
		}
		chunkReader.Close()
	}
}
