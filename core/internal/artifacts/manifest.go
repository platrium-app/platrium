package artifacts

import (
	"platrium/internal/domain"

	"github.com/google/uuid"
)

type ManifestArtifact struct {
	baseArtifact
}

func CreateManifestArtifact(id uuid.UUID) (*ManifestArtifact, error) {
	manifestArtifact := &ManifestArtifact{
		baseArtifact: baseArtifact{
			id:           id.String(),
			artifactType: domain.ManifestArtifactType,
		},
	}

	return manifestArtifact, nil
}
