package identity

import (
	"encoding/json"
	"net/http"
)

type TenantHandler struct {
	tenantStore *TenantStore
}

func NewTenantHandler(tenantStore *TenantStore) *TenantHandler {
	return &TenantHandler{
		tenantStore: tenantStore,
	}
}

type createTenantRequest struct {
	Name string `json:"name"`
}

func (h *TenantHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest

	// TODO: use Go Validator.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "tenant name is required", http.StatusBadRequest)
		return
	}

	tenant, err := h.tenantStore.CreateTenant(r.Context(), req.Name)
	if err != nil {
		http.Error(w, "failed to create tenant", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tenant)
}
