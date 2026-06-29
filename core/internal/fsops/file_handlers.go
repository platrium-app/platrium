package fsops

import (
	"encoding/json"
	"net/http"
)

type CreateFileRequest struct {
	TenantID string   `json:"tenant_id"` // TODO: Rip from JWT middleware later
	ParentID string   `json:"parent_id"`
	FileName string   `json:"file_name"`
	Hashes   []string `json:"hashes"`
}

type FileHandler struct {
	fsOps *FSOps
}

func NewFileHandler(fsOps *FSOps) *FileHandler {
	return &FileHandler{fsOps: fsOps}
}

// CreateFile executes the full Hybrid Manifest File Creation flow.
func (h *FileHandler) CreateFile(w http.ResponseWriter, r *http.Request) {
	var req CreateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fileId, err := h.fsOps.CreateFile(r.Context(), req.TenantID, req.ParentID, req.FileName, req.Hashes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"file_id": fileId})
}
