//! Authentication calls for native clients.
//!
//! Thin, typed wrappers over the generated REST client. Because the records
//! below are built field by field from the generated models, a change to the
//! engine's API breaks this crate's build, and through UniFFI the Swift and
//! Kotlin apps, instead of failing at runtime.

use platrium_restapi::apis::configuration::Configuration;
use platrium_restapi::apis::{Error, default_api};
use platrium_restapi::models;
use std::sync::Arc;
use std::time::SystemTime;

use crate::errors::PlatriumError;

/// A freshly issued bearer token. The secret is only available here.
#[derive(Clone, Debug, uniffi::Record)]
pub struct TokenGrant {
    pub token: String,
    /// Server-side id of the token, used to list or revoke it.
    pub token_id: String,
    /// Set when the client was registered as a device.
    pub device_id: Option<String>,
    pub expires_at: Option<SystemTime>,
}

/// How a request authenticated.
#[derive(Clone, Copy, Debug, PartialEq, Eq, uniffi::Enum)]
pub enum AuthKind {
    Session,
    Device,
    App,
}

/// Who a token acts as.
#[derive(Clone, Debug, uniffi::Record)]
pub struct Identity {
    pub user_id: String,
    pub tenant_id: String,
    pub email: String,
    pub kind: AuthKind,
    pub device_id: Option<String>,
}

#[derive(uniffi::Object)]
pub struct AuthApi {
    api_config: Arc<Configuration>,
}

impl AuthApi {
    pub(crate) fn new(api_config: Arc<Configuration>) -> Self {
        Self { api_config }
    }
}

#[uniffi::export(async_runtime = "tokio")]
impl AuthApi {
    /// Redeems the single-use code from the browser sign-in together with the
    /// PKCE `code_verifier`. Works on a client without a token.
    pub async fn exchange_code(
        &self,
        code: String,
        code_verifier: String,
    ) -> Result<TokenGrant, PlatriumError> {
        let req = models::AuthTokenRequest::new(code, code_verifier);
        let resp = default_api::auth_token(&self.api_config, req)
            .await
            .map_err(map_error)?;
        Ok(TokenGrant {
            token: resp.token,
            token_id: resp.id,
            device_id: resp.device_id,
            expires_at: resp.expires_at.map(SystemTime::from),
        })
    }

    /// Returns the identity behind this client's bearer token. A revoked or
    /// expired token fails with `PlatriumError::Unauthorized`.
    pub async fn me(&self) -> Result<Identity, PlatriumError> {
        let resp = default_api::auth_me(&self.api_config)
            .await
            .map_err(map_error)?;
        Ok(Identity {
            user_id: resp.user_id,
            tenant_id: resp.tenant_id,
            email: resp.email,
            kind: match resp.auth_kind {
                models::auth_auth_me_response::AuthKind::Session => AuthKind::Session,
                models::auth_auth_me_response::AuthKind::Device => AuthKind::Device,
                models::auth_auth_me_response::AuthKind::App => AuthKind::App,
            },
            device_id: resp.device_id,
        })
    }
}

/// Maps a generated-client error onto the SDK's error type, keeping the HTTP
/// statuses apps need to react to.
fn map_error<E: std::fmt::Debug>(err: Error<E>) -> PlatriumError {
    match err {
        Error::ResponseError(resp) => match resp.status.as_u16() {
            401 => PlatriumError::Unauthorized(resp.content),
            400 => PlatriumError::BadRequest(resp.content),
            _ => PlatriumError::ApiError(format!("HTTP {}: {}", resp.status, resp.content)),
        },
        other => PlatriumError::ApiError(other.to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::TcpListener;

    /// Serves one canned HTTP response and returns the base path to reach it.
    async fn serve_once(status: &str, body: &'static str) -> (String, tokio::task::JoinHandle<String>) {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let status = status.to_string();
        let handle = tokio::spawn(async move {
            let (mut sock, _) = listener.accept().await.unwrap();
            let mut buf = vec![0u8; 8192];
            let n = sock.read(&mut buf).await.unwrap();
            let request = String::from_utf8_lossy(&buf[..n]).to_string();
            let resp = format!(
                "HTTP/1.1 {status}\r\ncontent-type: application/json\r\ncontent-length: {}\r\nconnection: close\r\n\r\n{body}",
                body.len()
            );
            sock.write_all(resp.as_bytes()).await.unwrap();
            request
        });
        (format!("http://{addr}"), handle)
    }

    fn api(base: String) -> AuthApi {
        let mut cfg = Configuration::new();
        cfg.base_path = base;
        AuthApi::new(Arc::new(cfg))
    }

    #[tokio::test]
    async fn exchange_code_maps_the_grant() {
        let (base, req) = serve_once(
            "200 OK",
            r#"{"token":"plt_abc","id":"tok1","device_id":"dev1","expires_at":"2030-01-01T00:00:00Z"}"#,
        )
        .await;
        let grant = api(base).exchange_code("the-code".into(), "the-verifier".into()).await.unwrap();
        assert_eq!(grant.token, "plt_abc");
        assert_eq!(grant.token_id, "tok1");
        assert_eq!(grant.device_id.as_deref(), Some("dev1"));
        assert!(grant.expires_at.is_some());

        let request = req.await.unwrap();
        assert!(request.starts_with("POST /auth/token"), "{request}");
        assert!(request.contains(r#""code":"the-code""#) && request.contains(r#""code_verifier":"the-verifier""#));
    }

    #[tokio::test]
    async fn app_grant_has_no_device() {
        let (base, _) = serve_once("200 OK", r#"{"token":"plt_x","id":"t"}"#).await;
        let grant = api(base).exchange_code("c".into(), "v".into()).await.unwrap();
        assert!(grant.device_id.is_none() && grant.expires_at.is_none());
    }

    #[tokio::test]
    async fn bad_code_is_a_bad_request() {
        let (base, _) = serve_once("400 Bad Request", r#"{"message":"invalid_grant"}"#).await;
        let err = api(base).exchange_code("c".into(), "v".into()).await.unwrap_err();
        assert!(matches!(err, PlatriumError::BadRequest(ref m) if m.contains("invalid_grant")), "{err:?}");
    }

    #[tokio::test]
    async fn me_maps_the_identity() {
        let (base, _) = serve_once(
            "200 OK",
            r#"{"user_id":"u1","tenant_id":"t1","email":"a@b.c","auth_kind":"DEVICE","device_id":"d1"}"#,
        )
        .await;
        let me = api(base).me().await.unwrap();
        assert_eq!((me.user_id.as_str(), me.tenant_id.as_str(), me.email.as_str()), ("u1", "t1", "a@b.c"));
        assert_eq!(me.kind, AuthKind::Device);
        assert_eq!(me.device_id.as_deref(), Some("d1"));
    }

    #[tokio::test]
    async fn revoked_token_is_unauthorized() {
        let (base, _) = serve_once("401 Unauthorized", r#"{"message":"Invalid or expired token"}"#).await;
        let err = api(base).me().await.unwrap_err();
        assert!(matches!(err, PlatriumError::Unauthorized(_)), "{err:?}");
    }
}
