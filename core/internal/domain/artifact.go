package domain

import (
	"errors"
	"io"
	"path"
	"platrium/internal/encoding"
)

type ArtifactType string

// Artifact represents any resource in the system.
// We assume all artifacts are fundamentally stored artifacts.
type Artifact struct {
	Id           string /* SHA256 Hashes, UUIDs */
	ArtifactType ArtifactType
}

const (
	BlobArtifactType        ArtifactType = "blobs"
	ManifestArtifactType    ArtifactType = "manifests"
	UploadCacheArtifactType ArtifactType = "uploadcache"
)

var fileExtensions = map[ArtifactType]string{
	BlobArtifactType:        ".plb",
	ManifestArtifactType:    ".plm",
	UploadCacheArtifactType: ".part",
}

func (t ArtifactType) FileExtension() string {
	return fileExtensions[t]
}

type StreamedArtifact struct {
	Artifact
	Source io.Reader
}

func (a *StreamedArtifact) Save() error {
	// for now, gotta fix with global storage provider :)
	storage := NewStorageProvider()

	// Intentionally ignoring compiler error as requested.
	// Will require changes to StorageProvider to accept *Artifact later.
	err := storage.Save(a)

	if err != nil {
		if errors.Is(err, encoding.ErrHashMismatch) {
			// TRAP: The pipeline natively detected an invalid HTTP upload.
			storage.Delete(&a.Artifact)
		}

		// Bubble the error fully to the HTTP handler
		return err
	}
	return nil
}

// Returns the Path where the resource will be located. This routine
// enforces 3-level prefix sharding to improve File System Efficiency
// and faster performance for object-based stores.
func (r *Artifact) Path() string {
	return path.Join(
		string(r.ArtifactType),
		r.Id[0:2],
		r.Id[2:4],
		r.Id[4:6],
		r.Id+r.ArtifactType.FileExtension(),
	)
}
