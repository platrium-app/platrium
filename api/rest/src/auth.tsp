import "@typespec/http";
import "@typespec/openapi";
import "./errors.tsp";

using TypeSpec.Http;
using TypeSpec.OpenAPI;

@route("/auth")
namespace Platrium.Auth;

interface Auth {
    @route("/login")
    @post
    @summary("Local Login")
    @doc("Authenticates a user via credentials against a specific IdP and establishes a session or issues a challenge.")
    local_user_login(@body req: LoginRequest):
        | LoginResponse
        | Errors.Unauthorized
        | Errors.EngineInternal;

    @route("/login")
    @get
    @summary("SSO Login Redirect")
    @doc("Initiates an external SSO handshake (SAML/OIDC) and redirects the user.")
    idp_redirect(@query idp: string):
        | SSORedirectResponse
        | Errors.NotFound
        | Errors.EngineInternal;

    @route("/mfa/verify")
    @post
    @summary("Verify MFA Challenge")
    @doc("Verifies an MFA challenge token and establishes a session.")
    verify(@body req: MFAVerifyRequest):
        | LoginResponse
        | Errors.Unauthorized
        | Errors.EngineInternal;

    @route("/me")
    @get
    @summary("Get Current User")
    @doc("Returns the currently authenticated user's session data.")
    me():
        | AuthMeResponse
        | Errors.Unauthorized;

    @route("/logout")
    @post
    @summary("Log Out")
    @doc("Ends the current credential: deletes the calling bearer token (and its device), or destroys the browser session.")
    logout():
        | NoContent
        | Errors.EngineInternal;

    @route("/authorize")
    @post
    @summary("Authorize a Client")
    @doc("Called by the web consent page once the signed-in user approves a native client or app. Stores a single-use code bound to the PKCE challenge and returns the URL to send the user back to. Browser session only. Include platform to register the client as a device.")
    authorize(@body req: AuthorizeRequest):
        | AuthorizeResponse
        | Errors.BadRequest
        | Errors.Unauthorized
        | Errors.Forbidden
        | Errors.EngineInternal;

    @route("/token")
    @post
    @summary("Exchange Code for Token")
    @doc("Redeems the single-use code from /authorize together with the PKCE code_verifier and returns a bearer token. Rate limited.")
    token(@body req: TokenRequest):
        | TokenResponse
        | Errors.BadRequest
        | Errors.EngineInternal;

    @route("/clients")
    @get
    @summary("List Devices and Apps")
    @doc("Lists the devices and apps signed in as the current user.")
    list_clients():
        | ClientListResponse
        | Errors.Unauthorized
        | Errors.EngineInternal;

    @route("/clients")
    @post
    @summary("Create App Token")
    @doc("Manually creates a named app token for scripts and CI. The token is only returned here. Browser session only.")
    create_client(@body req: CreateClientRequest):
        | TokenResponse
        | Errors.BadRequest
        | Errors.Unauthorized
        | Errors.Forbidden
        | Errors.EngineInternal;

    @route("/clients/{id}")
    @delete
    @summary("Revoke Device or App")
    @doc("Deletes the token and, for a device, the device with it. Takes effect immediately.")
    delete_client(@path id: string):
        | NoContent
        | Errors.Unauthorized
        | Errors.NotFound
        | Errors.EngineInternal;

    @route("/device/push")
    @put
    @summary("Register Push Token")
    @doc("Registers the APNs or FCM token for the device that owns the calling bearer token.")
    set_push(@body req: PushRequest):
        | NoContent
        | Errors.BadRequest
        | Errors.Unauthorized
        | Errors.Forbidden
        | Errors.EngineInternal;

    @route("/device/push")
    @delete
    @summary("Clear Push Token")
    @doc("Stops push delivery to the device that owns the calling bearer token.")
    clear_push():
        | NoContent
        | Errors.Unauthorized
        | Errors.Forbidden
        | Errors.EngineInternal;
}

model LoginRequest {
    idp_id: string;
    email: string;
    password?: string;
}

model LoginResponse {
    @statusCode _: 200;
    status: string; // "SUCCESS" | "CHALLENGE_REQUIRED"
    next_step?: string; // "MFA_TOTP" | "WEBAUTHN"
    challenge_token?: string;

    @minItems(1)
    available_methods?: string[];
}

model SSORedirectResponse {
    @statusCode _: 302;
    @header location: string;
}

model MFAVerifyRequest {
    challenge_token: string;
    method: string;
    code: string;
}

model AuthMeResponse {
    @statusCode _: 200;
    user_id: string;
    tenant_id: string;
    email: string;

    @doc("How this request authenticated: SESSION (browser), DEVICE or APP (bearer token).")
    auth_kind: "SESSION" | "DEVICE" | "APP";

    @doc("Set when auth_kind is DEVICE.")
    device_id?: string;
}

model NoContent {
    @statusCode _: 204;
}

model AuthorizeRequest {
    @doc("Where to send the user afterwards: a custom URL scheme, a loopback http URL (127.0.0.1, [::1], localhost, any port), or a configured https universal link.")
    redirect_uri: string;

    @doc("Base64url SHA-256 of the client's secret code_verifier (PKCE S256).")
    code_challenge: string;

    @doc("Opaque client value echoed back on the redirect.")
    state?: string;

    @doc("Shown to the user and in the Devices/Apps list, e.g. \"Platrium for iPhone\" or \"Platrium CLI\".")
    name: string;

    @doc("IOS, MACOS, ANDROID, ... When present the client is registered as a device; when absent it is an app.")
    platform?: string;

    app_version?: string;
}

model AuthorizeResponse {
    @statusCode _: 200;
    redirect_to: string;
}

model TokenRequest {
    code: string;
    code_verifier: string;
}

model TokenResponse {
    @statusCode _: 200;
    @doc("Bearer token. Only shown once.")
    token: string;
    @doc("Token ID, usable with DELETE /auth/clients/{id}.")
    id: string;
    device_id?: string;
    expires_at?: utcDateTime;
}

model Client {
    id: string;
    kind: "DEVICE" | "APP";
    name: string;
    platform?: string;
    app_version?: string;
    created_at: utcDateTime;
    last_used_at: utcDateTime;
    expires_at?: utcDateTime;
    @doc("True for the credential making this request.")
    current: boolean;
}

model ClientListResponse {
    @statusCode _: 200;
    clients: Client[];
}

model CreateClientRequest {
    name: string;
    @doc("Lifetime in days; omit for a token that only expires when idle.")
    expires_in_days?: int32;
}

model PushRequest {
    transport: "APNS" | "FCM";
    token: string;
}
