package restapi

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"platrium/internal/auth/session"
	"platrium/internal/auth/token"
	"platrium/internal/identity"
)

const (
	maxNameLen       = 255
	maxStateLen      = 512
	maxAppVersionLen = 64
	maxPushTokenLen  = 1024
	maxTokenDays     = 3650
)

var platformRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)

func str(s string) *string { return &s }

func internalErr(what string, err error) string {
	log.Printf("%s: %v", what, err)
	return what
}

// callerAuth is the authenticated caller of a request.
type callerAuth struct {
	sess *session.PlatriumSession
	info session.AuthInfo
}

func authFrom(ctx context.Context) (callerAuth, bool) {
	sess, ok := session.FromContext(ctx)
	if !ok {
		return callerAuth{}, false
	}
	info, _ := session.AuthInfoFromContext(ctx)
	return callerAuth{sess: sess, info: info}, true
}

// AuthAuthorize implements POST /auth/authorize.
func (a *RestAPI) AuthAuthorize(ctx context.Context, request AuthAuthorizeRequestObject) (AuthAuthorizeResponseObject, error) {
	caller, ok := authFrom(ctx)
	if !ok {
		return AuthAuthorize401JSONResponse{Message: str("Sign in to continue")}, nil
	}
	// Only a person at a browser can approve a client; a token must not be
	// able to mint more tokens.
	if caller.info.Kind != session.AuthKindSession {
		return AuthAuthorize403JSONResponse{Message: str("Authorization requires a browser session")}, nil
	}

	req := request.Body
	if req == nil {
		return AuthAuthorize400JSONResponse{Message: str("Missing request body")}, nil
	}
	redirect, err := token.ParseRedirect(req.RedirectUri, a.nativeHTTPSRedirects)
	if err != nil {
		return AuthAuthorize400JSONResponse{Message: str("redirect_uri is not allowed")}, nil
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > maxNameLen {
		return AuthAuthorize400JSONResponse{Message: str("name is required")}, nil
	}
	// A SHA-256 digest is 32 bytes, 43 characters of unpadded base64url.
	if b, err := base64.RawURLEncoding.DecodeString(req.CodeChallenge); err != nil || len(b) != 32 {
		return AuthAuthorize400JSONResponse{Message: str("code_challenge must be a base64url SHA-256 digest")}, nil
	}
	state := derefStr(req.State)
	if len(state) > maxStateLen {
		return AuthAuthorize400JSONResponse{Message: str("state is too long")}, nil
	}
	grant := token.Grant{
		TenantID:   caller.sess.TenantID,
		UserID:     caller.sess.UserID,
		Name:       name,
		AppVersion: derefStr(req.AppVersion),
		Challenge:  req.CodeChallenge,
	}
	if len(grant.AppVersion) > maxAppVersionLen {
		return AuthAuthorize400JSONResponse{Message: str("app_version is too long")}, nil
	}
	if req.Platform != nil {
		grant.Platform = strings.ToUpper(strings.TrimSpace(*req.Platform))
		if !platformRe.MatchString(grant.Platform) {
			return AuthAuthorize400JSONResponse{Message: str("platform is invalid")}, nil
		}
	}

	code, err := a.CodeStore.Create(ctx, grant)
	if err != nil {
		return AuthAuthorize500JSONResponse{Debuginfo: internalErr("failed to create authorization code", err)}, nil
	}

	q := redirect.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	redirect.RawQuery = q.Encode()
	return AuthAuthorize200JSONResponse{RedirectTo: redirect.String()}, nil
}

// AuthToken implements POST /auth/token.
func (a *RestAPI) AuthToken(ctx context.Context, request AuthTokenRequestObject) (AuthTokenResponseObject, error) {
	if request.Body == nil {
		return AuthToken400JSONResponse{Message: str("Missing request body")}, nil
	}
	grant, err := a.CodeStore.Redeem(ctx, request.Body.Code, request.Body.CodeVerifier)
	if errors.Is(err, token.ErrInvalidCode) {
		return AuthToken400JSONResponse{Message: str("invalid_grant")}, nil
	}
	if err != nil {
		return AuthToken500JSONResponse{Debuginfo: internalErr("failed to redeem code", err)}, nil
	}

	req := token.IssueReq{TenantID: grant.TenantID, UserID: grant.UserID, Name: grant.Name}
	if grant.Platform != "" {
		req.Device = &identity.RegisterDeviceReq{Name: grant.Name, Platform: grant.Platform, AppVersion: grant.AppVersion}
	}
	issued, err := a.TokenStore.Issue(ctx, req)
	if errors.Is(err, identity.ErrNotFound) {
		// The user was removed between approval and exchange.
		return AuthToken400JSONResponse{Message: str("invalid_grant")}, nil
	}
	if err != nil {
		return AuthToken500JSONResponse{Debuginfo: internalErr("failed to issue token", err)}, nil
	}
	return AuthToken200JSONResponse(tokenResponse(issued)), nil
}

// AuthListClients implements GET /auth/clients.
func (a *RestAPI) AuthListClients(ctx context.Context, request AuthListClientsRequestObject) (AuthListClientsResponseObject, error) {
	caller, ok := authFrom(ctx)
	if !ok {
		return AuthListClients401JSONResponse{}, nil
	}
	clients, err := a.TokenStore.List(ctx, caller.sess.TenantID, caller.sess.UserID)
	if err != nil {
		return AuthListClients500JSONResponse{Debuginfo: internalErr("failed to list clients", err)}, nil
	}
	out := make([]AuthClient, 0, len(clients))
	for _, c := range clients {
		item := AuthClient{
			Id: c.ID, Name: c.Name, Kind: AuthClientKindAPP,
			CreatedAt: c.CreatedAt, LastUsedAt: c.LastUsedAt, ExpiresAt: c.ExpiresAt,
			Current: caller.info.TokenID != "" && caller.info.TokenID == c.ID,
		}
		if c.IsDevice() {
			item.Kind = AuthClientKindDEVICE
			item.Platform = str(c.Platform)
			if c.AppVersion != "" {
				item.AppVersion = str(c.AppVersion)
			}
		}
		out = append(out, item)
	}
	return AuthListClients200JSONResponse{Clients: out}, nil
}

// AuthCreateClient implements POST /auth/clients.
func (a *RestAPI) AuthCreateClient(ctx context.Context, request AuthCreateClientRequestObject) (AuthCreateClientResponseObject, error) {
	caller, ok := authFrom(ctx)
	if !ok {
		return AuthCreateClient401JSONResponse{}, nil
	}
	if caller.info.Kind != session.AuthKindSession {
		return AuthCreateClient403JSONResponse{Message: str("Tokens can only be created from a browser session")}, nil
	}
	req := request.Body
	if req == nil {
		return AuthCreateClient400JSONResponse{Message: str("Missing request body")}, nil
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > maxNameLen {
		return AuthCreateClient400JSONResponse{Message: str("name is required")}, nil
	}
	issue := token.IssueReq{TenantID: caller.sess.TenantID, UserID: caller.sess.UserID, Name: name}
	if req.ExpiresInDays != nil {
		days := int(*req.ExpiresInDays)
		if days < 1 || days > maxTokenDays {
			return AuthCreateClient400JSONResponse{Message: str("expires_in_days is out of range")}, nil
		}
		exp := time.Now().UTC().AddDate(0, 0, days)
		issue.ExpiresAt = &exp
	}
	issued, err := a.TokenStore.Issue(ctx, issue)
	if err != nil {
		return AuthCreateClient500JSONResponse{Debuginfo: internalErr("failed to create token", err)}, nil
	}
	return AuthCreateClient200JSONResponse(tokenResponse(issued)), nil
}

// AuthDeleteClient implements DELETE /auth/clients/{id}.
func (a *RestAPI) AuthDeleteClient(ctx context.Context, request AuthDeleteClientRequestObject) (AuthDeleteClientResponseObject, error) {
	caller, ok := authFrom(ctx)
	if !ok {
		return AuthDeleteClient401JSONResponse{}, nil
	}
	err := a.TokenStore.Delete(ctx, caller.sess.TenantID, caller.sess.UserID, request.Id)
	if errors.Is(err, identity.ErrNotFound) {
		return AuthDeleteClient404JSONResponse{Message: str("Client not found")}, nil
	}
	if err != nil {
		return AuthDeleteClient500JSONResponse{Debuginfo: internalErr("failed to delete client", err)}, nil
	}
	return AuthDeleteClient204Response{}, nil
}

// AuthSetPush implements PUT /auth/device/push.
func (a *RestAPI) AuthSetPush(ctx context.Context, request AuthSetPushRequestObject) (AuthSetPushResponseObject, error) {
	caller, ok := authFrom(ctx)
	if !ok {
		return AuthSetPush401JSONResponse{}, nil
	}
	if caller.info.Kind != session.AuthKindDevice {
		return AuthSetPush403JSONResponse{Message: str("Push registration requires a device token")}, nil
	}
	req := request.Body
	if req == nil || !req.Transport.Valid() || req.Token == "" || len(req.Token) > maxPushTokenLen {
		return AuthSetPush400JSONResponse{Message: str("transport and token are required")}, nil
	}
	if err := a.DeviceStore.SetPush(ctx, caller.sess.TenantID, caller.info.DeviceID, string(req.Transport), req.Token); err != nil {
		return AuthSetPush500JSONResponse{Debuginfo: internalErr("failed to register push token", err)}, nil
	}
	return AuthSetPush204Response{}, nil
}

// AuthClearPush implements DELETE /auth/device/push.
func (a *RestAPI) AuthClearPush(ctx context.Context, request AuthClearPushRequestObject) (AuthClearPushResponseObject, error) {
	caller, ok := authFrom(ctx)
	if !ok {
		return AuthClearPush401JSONResponse{}, nil
	}
	if caller.info.Kind != session.AuthKindDevice {
		return AuthClearPush403JSONResponse{Message: str("Push registration requires a device token")}, nil
	}
	if err := a.DeviceStore.ClearPush(ctx, caller.sess.TenantID, caller.info.DeviceID); err != nil {
		return AuthClearPush500JSONResponse{Debuginfo: internalErr("failed to clear push token", err)}, nil
	}
	return AuthClearPush204Response{}, nil
}

// AuthLogout implements POST /auth/logout.
func (a *RestAPI) AuthLogout(ctx context.Context, request AuthLogoutRequestObject) (AuthLogoutResponseObject, error) {
	if caller, ok := authFrom(ctx); ok && caller.info.Kind != session.AuthKindSession {
		err := a.TokenStore.Delete(ctx, caller.sess.TenantID, caller.sess.UserID, caller.info.TokenID)
		if err != nil && !errors.Is(err, identity.ErrNotFound) {
			return AuthLogout500JSONResponse{Debuginfo: internalErr("failed to revoke token", err)}, nil
		}
		return AuthLogout204Response{}, nil
	}
	if err := a.SessionManager.Destroy(ctx); err != nil {
		return AuthLogout500JSONResponse{Debuginfo: internalErr("failed to destroy session", err)}, nil
	}
	return AuthLogout204Response{}, nil
}

func tokenResponse(i *token.Issued) AuthTokenResponse {
	r := AuthTokenResponse{Id: i.ID, Token: i.Secret, ExpiresAt: i.ExpiresAt}
	if i.DeviceID != "" {
		r.DeviceId = str(i.DeviceID)
	}
	return r
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
