package artifacts

import (
	"io"
	"platrium/internal/domain"
)

type UploadCacheArtifact struct {
	baseArtifact
	source io.Reader
}

func NewUploadCacheArtifact(hexHash string, src io.Reader) *UploadCacheArtifact {
	return &UploadCacheArtifact{
		source: src,
		baseArtifact: baseArtifact{
			id:           hexHash,
			artifactType: domain.UploadCacheArtifactType,
		},
	}
}

func (u *UploadCacheArtifact) Reader() (io.Reader, error) {
	return u.source, nil
}

var _ domain.StoredArtifact = (*UploadCacheArtifact)(nil)
