package artifacts

import "platrium/internal/domain"

type BlobArtifact struct {
	domain.StreamedArtifact
}

// we cant create these direcrtly, only get this artifact!!
