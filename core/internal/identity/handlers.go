package identity

import (
	"encoding/json"
	"net/http"

	"platrium/internal/fsops"
)

type TenantHandler struct {
	tenantStore *TenantStore
	userStore   *UserStore
	fsOps       *fsops.FSOps
}

func NewTenantHandler(tenantStore *TenantStore, userStore *UserStore, fsOps *fsops.FSOps) *TenantHandler {
	return &TenantHandler{
		tenantStore: tenantStore,
		userStore:   userStore,
		fsOps:       fsOps,
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

	// Hardcoded default admin for the tenant
	user, err := h.userStore.CreateUser(r.Context(), tenant.ID, "admin@example.com")
	if err != nil {
		http.Error(w, "failed to create tenant super admin", http.StatusInternalServerError)
		return
	}

	// Hardcode a private drive creation for the user (to be moved to separate flow later)
	drive, err := h.fsOps.CreatePrivateDrive(r.Context(), tenant.ID, user.ID)
	if err != nil {
		http.Error(w, "failed to create private drive for admin", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenant": tenant,
		"admin":  user,
		"drive":  drive,
	})
}
