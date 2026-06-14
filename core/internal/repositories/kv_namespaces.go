package repositories

// Centralized KV namespaces to guarantee no collision occurs across different repositories.
const (
	NamespaceAttachedFSWrites = "afsw"
	// Future namespaces:
	// NamespaceManifest    = "mfst"
	// NamespaceChunkStatus = "chnk"
)
