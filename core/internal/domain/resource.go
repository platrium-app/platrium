package domain

import (
	"io"
)

type ArtifactType string

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

type StoredArtifact interface {
	ID() string
	Type() ArtifactType
	Path() string
	Reader() (io.Reader, error)
}
