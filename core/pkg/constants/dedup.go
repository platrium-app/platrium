package constants

const (
	// DedupChunkSizeBytes defines the globally enforced 4MB chunk size for all uploads.
	// This is a strict protocol constant shared across all clients and servers in the monorepo.
	DedupChunkSizeBytes = 4 * 1024 * 1024
)
