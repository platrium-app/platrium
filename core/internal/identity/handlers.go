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
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

func (h *TenantHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest

	// TODO: use Go Validator.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Alias == "" {
		http.Error(w, "tenant name and alias are required", http.StatusBadRequest)
		return
	}

	if len(req.Alias) < 2 {
		http.Error(w, "tenant alias must be at least 2 characters", http.StatusBadRequest)
		return
	}

	tenant, err := h.tenantStore.CreateTenant(r.Context(), req.Name, req.Alias)
	if err != nil {
		http.Error(w, "failed to create tenant", http.StatusInternalServerError)
		return
	}

	// TODO: Hardcoded default admin for the tenant
	user, err := h.userStore.CreateUser(r.Context(), tenant.ID, "admin@example.com")
	if err != nil {
		http.Error(w, "failed to create tenant super admin", http.StatusInternalServerError)
		return
	}

	// TODO: Hardcode a private drive creation for the user (to be moved to separate flow later)
	drive, err := h.fsOps.CreateDrive(r.Context(), fsops.CreateDriveParams{
		TenantID: tenant.ID,
		OwnerID:  user.ID,
		Name:     "My Drive",
		Type:     fsops.DriveTypePrivate,
	})

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
