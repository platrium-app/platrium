#[derive(Debug, uniffi::Error)]
pub enum PlatriumError {
    ApiError(String),
    InternalError(String),
}

impl std::fmt::Display for PlatriumError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            PlatriumError::ApiError(msg) => write!(f, "API Error: {}", msg),
            PlatriumError::InternalError(msg) => write!(f, "Internal Error: {}", msg),
        }
    }
}

impl std::error::Error for PlatriumError {}
