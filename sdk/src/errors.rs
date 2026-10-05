#[derive(Debug, uniffi::Error)]
pub enum PlatriumError {
    /// The server rejected the credentials (HTTP 401): the token is missing, revoked or expired.
    Unauthorized(String),
    /// The server rejected the request (HTTP 400), e.g. an invalid or already-used authorization code.
    BadRequest(String),
    ApiError(String),
    InternalError(String),
}

impl std::fmt::Display for PlatriumError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            PlatriumError::Unauthorized(msg) => write!(f, "Unauthorized: {}", msg),
            PlatriumError::BadRequest(msg) => write!(f, "Bad Request: {}", msg),
            PlatriumError::ApiError(msg) => write!(f, "API Error: {}", msg),
            PlatriumError::InternalError(msg) => write!(f, "Internal Error: {}", msg),
        }
    }
}

impl std::error::Error for PlatriumError {}

#[cfg(target_arch = "wasm32")]
impl From<PlatriumError> for wasm_bindgen::JsValue {
    fn from(err: PlatriumError) -> Self {
        wasm_bindgen::JsValue::from_str(&err.to_string())
    }
}
