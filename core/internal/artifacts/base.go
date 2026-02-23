package artifacts

import (
	"path"
	"platrium/internal/domain"
)

type baseArtifact struct {
	id           string /* SHA256 Hashes, UUIDs */
	artifactType domain.ArtifactType
}

func (r *baseArtifact) ID() string                { return r.id }
func (r *baseArtifact) Type() domain.ArtifactType { return r.artifactType }

// Returns the Path where the resource will be located. This routine
// enforces 3-level prefix sharding to improve File System Efficiency
// and faster performance for object-based stores.
func (r *baseArtifact) Path() string {
	return path.Join(
		string(r.artifactType),
		r.id[0:2],
		r.id[2:4],
		r.id[4:6],
		r.id+r.artifactType.FileExtension(),
	)
}
