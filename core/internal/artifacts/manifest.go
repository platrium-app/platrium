package artifacts

import (
	"platrium/internal/domain"

	"github.com/google/uuid"
)

type ManifestArtifact struct {
	domain.StreamedArtifact
}

func CreateManifestArtifact(id uuid.UUID) (*ManifestArtifact, error) {
	manifestArtifact := &ManifestArtifact{
		StreamedArtifact: domain.StreamedArtifact{
			Source: nil,
			Artifact: domain.Artifact{
				Id:           id.String(),
				ArtifactType: domain.ManifestArtifactType,
			},
		},
	}

	return manifestArtifact, nil
}
