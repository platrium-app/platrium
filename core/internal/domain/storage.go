package domain

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

// make this a struct and impl directly. We'll use another StorageBackend iface for backends.
type StorageProvider interface {
	Save(ctx context.Context, res StoredArtifact) error
	Load(ctx context.Context, res StoredArtifact) error
	Exists(ctx context.Context, res StoredArtifact) (bool, error)
}

type Sb struct {
}

func NewSb() *Sb {
	return &Sb{}
}

func (s *Sb) Save(res StoredArtifact) {
	reader, err := res.Reader()
	if err != nil {
		log.Fatalf("Failed to Get Reader for UploadCacheArtifact %s", res.ID())
		return
	}

	written, err := io.Copy(os.Stdout, reader)
	if err != nil {
		log.Fatalf("Failed to Write for UploadCacheArtifact %s", res.ID())
		return
	}

	fmt.Printf("\n--- Done! Total bytes streamed to console: %d ---\n", written)
}
