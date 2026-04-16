package providers

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"platrium/internal/domain"
)

// make this a struct and impl directly. We'll use another StorageBackend iface for backends.
// type StorageProvider interface {
// 	Save(ctx context.Context, res StoredArtifact) error
// 	Load(ctx context.Context, res StoredArtifact) error
// 	Exists(ctx context.Context, res StoredArtifact) (bool, error)
// }

type StorageProvider struct {
}

func NewStorageProvider() *StorageProvider {
	return &StorageProvider{}
}

func (s *StorageProvider) Save(artifact *domain.StreamedArtifact) error {
	absolutePath, err := resolveLocalPath(artifact.Path())
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(absolutePath)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create nested directories at %s: %w", targetDir, err)
	}

	file, err := os.Create(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to create file at %s: %w", absolutePath, err)
	}

	defer file.Close()

	written, err := io.Copy(file, artifact.Source)
	if err != nil {
		log.Printf("Storage stream explicitly aborted for %s (wrote %d bytes): %v", absolutePath, written, err)
		return err
	}

	fmt.Printf("\n--- Done! Total bytes Written: %d ---\n", written)
	fmt.Printf("Path: %s", absolutePath)
	return nil
}

func (s *StorageProvider) Delete(artifact *domain.Artifact) error {
	absolutePath, err := resolveLocalPath(artifact.Path())
	if err != nil {
		return err
	}

	fmt.Printf("Deleting File from Backend %s at path %s\n", "local", absolutePath)
	return os.Remove(absolutePath)
}

func (s *StorageProvider) Load(artifact *domain.Artifact) (io.ReadCloser, error) {
	absolutePath, err := resolveLocalPath(artifact.Path())
	if err != nil {
		return nil, err
	}

	file, err := os.Open(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open artifact file at %s: %w", absolutePath, err)
	}

	return file, nil
}

// func (s *StorageProvider) Move(oldLoc *Artifact, newLoc *Artifact) error {
// 	fmt.Println("File Moved")
// 	return nil
// }

// resolveLocalPath resolves a relative artifact path to its absolute location
// within the local file system's "data/" directory.
// This logic will eventually reside exclusively within the future `local` StorageBackend implementation.
func resolveLocalPath(relativePath string) (string, error) {
	execdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get executable file path: %w", err)
	}
	basePath := filepath.Join(execdir, "data")
	return filepath.Join(basePath, relativePath), nil
}
