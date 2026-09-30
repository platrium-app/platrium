package flows

import (
	"net/http"
	"platrium/internal/auth"
)

// DeviceHandler handles the OAuth 2.0 Device Authorization Grant (RFC 8628).
// This is used for CLI logins, Desktop apps, or devices without a browser.
type DeviceHandler struct {
	authManager auth.AuthManager
}

func NewDeviceHandler(authManager auth.AuthManager) *DeviceHandler {
	return &DeviceHandler{
		authManager: authManager,
	}
}

// Authorize handles `POST /auth/device/authorize`
// It returns a user_code and device_code.
func (h *DeviceHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	// 1. Generate NanoID for user_code (e.g. "ABCD-1234") and device_code
	// 2. Save mapping in KV Store (Badger) with a short TTL (e.g. 15 minutes)
	// 3. Return JSON with the codes and verification_uri
}

// Token handles `POST /auth/device/token`
// The device aggressively polls this endpoint until the user completes the flow.
func (h *DeviceHandler) Token(w http.ResponseWriter, r *http.Request) {
	// 1. Validate device_code against Badger
	// 2. Check if the status is still "pending" (return 400 authorization_pending)
	// 3. If "approved", return the long-lived Session Token/JWT.
}
