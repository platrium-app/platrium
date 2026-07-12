package restapi

import (
	"encoding/json"
	"net/http"
)

// FilesCreateFile implements the OpenAPI POST /files/create endpoint.
func (api *RestAPI) FilesCreateFile(w http.ResponseWriter, r *http.Request) {
	var req CreateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: Hardcoded tenant ID for now, replace with JWT middleware later
	tenantId := "f6105961-cd9f-492a-8657-33f7e14ff1b2"

	fileId, err := api.FSOps.CreateFile(r.Context(), tenantId, req.ParentId, req.FileName, req.Hashes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateFileResponse{FileId: fileId})
}
