package artifacts

import (
	"encoding/hex"
	"fmt"
	"platrium/internal/domain"
)

type BlobArtifact struct {
	baseArtifact
}

func CreateBlobArtifact(hash string) (*BlobArtifact, error) {
	if len(hash) != 64 {
		return nil, fmt.Errorf("Invalid SHA256 Hash Length")
	}

	if _, err := hex.DecodeString(hash); err != nil {
		return nil, fmt.Errorf("Invalid SHA256 Hash Characters")
	}

	blobArtifact := &BlobArtifact{
		baseArtifact: baseArtifact{
			id:           hash,
			artifactType: domain.BlobArtifactType,
		},
	}

	return blobArtifact, nil
}
