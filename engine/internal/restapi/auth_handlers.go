package restapi

import (
	"context"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/auth/session"
)

// AuthLocalUserLogin implements the POST /auth/login endpoint
func (a *RestAPI) AuthLocalUserLogin(ctx context.Context, request AuthLocalUserLoginRequestObject) (AuthLocalUserLoginResponseObject, error) {
	idp, err := a.IdpStore.GetIdpById(ctx, request.Body.IdpId)
	if err != nil {
		msg := "Invalid Identity Provider"
		return AuthLocalUserLogin401JSONResponse{
			Message: &msg,
		}, nil
	}

	if idp.Type != "LOCAL" {
		msg := "Identity Provider does not support password authentication"
		return AuthLocalUserLogin401JSONResponse{
			Message: &msg,
		}, nil
	}

	user, tenantId, err := a.UserStore.GetUserByExternalId(ctx, idp.ID, local.NormalizeLogin(request.Body.Email))
	if err != nil {
		msg := "Invalid credentials"
		return AuthLocalUserLogin401JSONResponse{
			Message: &msg,
		}, nil
	}

	valid, err := a.LocalUserStore.VerifyPassword(ctx, user.ID, *request.Body.Password)
	if err != nil || !valid {
		msg := "Invalid credentials"
		return AuthLocalUserLogin401JSONResponse{
			Message: &msg,
		}, nil
	}

	if user.Disabled() {
		msg := "This account has been disabled. Contact your administrator."
		return AuthLocalUserLogin401JSONResponse{
			Message: &msg,
		}, nil
	}

	// Login successful!
	// Generate the session JWT (Wait, SCS uses an opaque session ID cookie with data in memory/KV).
	// Let's store the PlatriumSession in SCS!
	sess := &session.PlatriumSession{
		UserID:   user.ID,
		TenantID: tenantId,
		Email:    user.Email,
	}

	a.SessionManager.Put(ctx, session.StoreKey, sess)
	// Optionally renew the token for sliding expiration
	a.SessionManager.RenewToken(ctx)

	return AuthLocalUserLogin200JSONResponse{
		Status: "SUCCESS",
	}, nil
}

// AuthIdpRedirect implements the GET /auth/login endpoint
func (a *RestAPI) AuthIdpRedirect(ctx context.Context, request AuthIdpRedirectRequestObject) (AuthIdpRedirectResponseObject, error) {
	idp, err := a.IdpStore.GetIdpById(ctx, request.Params.Idp)
	if err != nil {
		msg := "Identity Provider not found"
		return AuthIdpRedirect404JSONResponse{
			Message: &msg,
		}, nil
	}

	if idp.Type == "LOCAL" {
		msg := "Identity Provider does not support SSO redirection"
		return AuthIdpRedirect404JSONResponse{ // Or 400, but TypeSpec says 404 for missing SSO config
			Message: &msg,
		}, nil
	}

	// TODO: Phase 2 - Generate SAML/OIDC Auth URL using idp.ProtoConfig
	idpRedirectUrl := "https://example.com/sso"

	return AuthIdpRedirect302Response{
		Headers: AuthIdpRedirect302ResponseHeaders{
			Location: idpRedirectUrl,
		},
	}, nil
}

// AuthVerify implements the POST /auth/mfa/verify endpoint
func (a *RestAPI) AuthVerify(ctx context.Context, request AuthVerifyRequestObject) (AuthVerifyResponseObject, error) {
	// TODO: Phase 2 - Implement MFA verification (TOTP/WebAuthn)
	return AuthVerify200JSONResponse{
		Status: "SUCCESS",
	}, nil
}

// AuthMe implements the GET /auth/me endpoint
func (a *RestAPI) AuthMe(ctx context.Context, request AuthMeRequestObject) (AuthMeResponseObject, error) {
	sess, ok := session.FromContext(ctx)
	if !ok {
		return AuthMe401JSONResponse{}, nil
	}

	// Browser cookies outlive a disabled account, so check it here too.
	if users, err := a.UserStore.GetByIDs(ctx, sess.TenantID, []string{sess.UserID}); err != nil {
		return nil, err
	} else if u, ok := users[sess.UserID]; !ok || u.Disabled() {
		return AuthMe401JSONResponse{}, nil
	}

	info, _ := session.AuthInfoFromContext(ctx)
	resp := AuthMe200JSONResponse{
		UserId:   sess.UserID,
		TenantId: sess.TenantID,
		Email:    sess.Email,
		AuthKind: AuthAuthMeResponseAuthKind(info.Kind),
	}
	if info.DeviceID != "" {
		resp.DeviceId = &info.DeviceID
	}
	return resp, nil
}
