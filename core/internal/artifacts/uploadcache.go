package artifacts

import (
	"io"
	"platrium/internal/domain"
	"platrium/internal/encoding"
)

type UploadCacheArtifact struct {
	domain.StreamedArtifact
}

// NewUploadCacheArtifact is called when a fresh HTTP upload arrives.
// It applies the rigorous PLB1 encoding and transformation pipeline on the fly.
func NewUploadCacheArtifact(hexHash string, src io.Reader, opts encoding.BlobEncodingOptions) (*UploadCacheArtifact, error) {
	// Build the secure Blob stream perfectly wrapped with its PLB1 header
	secureStream, err := encoding.EncodeToBlobStream(hexHash, src, opts)
	if err != nil {
		return nil, err
	}

	return &UploadCacheArtifact{
		StreamedArtifact: domain.StreamedArtifact{
			Source: secureStream,
			Artifact: domain.Artifact{
				Id:           hexHash,
				ArtifactType: domain.UploadCacheArtifactType,
			},
		},
	}, nil
}

func GetUploadCacheArtifact(hexHash string) (*UploadCacheArtifact, error) {
	artifact := domain.Artifact{
		Id:           hexHash,
		ArtifactType: domain.UploadCacheArtifactType,
	}

	storage := domain.NewStorageProvider()
	diskStream, err := storage.Load(&artifact)
	if err != nil {
		return nil, err
	}

	// Hand over the raw file handle to the Decoding Director
	_, decodedPayloadStream, err := encoding.DecodeFromBlobStream(diskStream)
	if err != nil {
		// Close the underlying file immediately if the decoding fails
		if closer, ok := diskStream.(io.Closer); ok {
			closer.Close()
		}
		return nil, err
	}

	return &UploadCacheArtifact{
		StreamedArtifact: domain.StreamedArtifact{
			Artifact: artifact,
			Source:   decodedPayloadStream,
		},
	}, nil
}

// Promotes an UploadCacheArtifact to a BlobArtifact after verifying
// hashes and sets the approprite file flags.
func (u *UploadCacheArtifact) Promote() (*BlobArtifact, error) {
	return nil, nil
}
