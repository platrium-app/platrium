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
}
