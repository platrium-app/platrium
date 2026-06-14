package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"

	"platrium/internal/pipelines"
	"platrium/internal/repositories"
)

type AttachedFSConfig struct {
	AppURL string `env:"APP_URL"`
}

const WriteCacheDir = "writecache"

// AttachedFSBackend implements StorageBackend for any OS-attached file system (local, RAID, NFS, etc.).
type AttachedFSBackend struct {
	writesRepo repositories.AttachedFSWritesRepository
	apiBaseURL string
}

// Ensure AttachedFSBackend implements StorageBackend
var _ StorageBackend = (*AttachedFSBackend)(nil)

// NewAttachedFSBackend initializes a new AttachedFSBackend.
func NewAttachedFSBackend(writesRepo repositories.AttachedFSWritesRepository) *AttachedFSBackend {
	var cfg AttachedFSConfig
	if err := env.Parse(&cfg); err != nil {
		panic(fmt.Sprintf("failed to parse AttachedFSConfig: %v", err)) // Fail fast if config is invalid
	}

	return &AttachedFSBackend{
		writesRepo: writesRepo,
		apiBaseURL: cfg.AppURL,
	}
}

// GenerateUploadURLs provisions temporary write sessions in KV and returns the local upload endpoints.
func (l *AttachedFSBackend) GenerateUploadURLs(ctx context.Context, location string, chunks map[string]ChunkUploadInfo) (map[string]string, error) {
	urls := make(map[string]string)
	for hash, info := range chunks {
		writeId := uuid.New().String()

		session := repositories.WriteSession{
			Location: location,
			Path:     info.Path,
		}

		err := l.writesRepo.SetUploadPath(ctx, writeId, session)
		if err != nil {
			return nil, fmt.Errorf("failed to create upload session: %w", err)
		}

		urls[hash] = fmt.Sprintf("%s/api/attachedfs/%s", l.apiBaseURL, writeId)
	}
	return urls, nil
}

// CommitLocalWrite safely streams the HTTP request body directly to disk, performing real-time cryptographic hash validation.
func (l *AttachedFSBackend) CommitLocalWrite(ctx context.Context, writeId string, stream io.Reader) error {
	session, err := l.writesRepo.GetUploadPath(ctx, writeId)
	if err != nil {
		return fmt.Errorf("invalid or expired write session: %w", err)
	}

	expectedHash := filepath.Base(session.Path)
	writeCachePath := filepath.Join(session.Location, WriteCacheDir, writeId)
	finalPath := filepath.Join(session.Location, session.Path)

	if err := os.MkdirAll(filepath.Dir(writeCachePath), 0755); err != nil {
		return fmt.Errorf("failed to create write cache dir: %w", err)
	}

	tmpFile, err := os.Create(writeCachePath)
	if err != nil {
		return fmt.Errorf("failed to create tmp file: %w", err)
	}

	var streamSuccess bool
	defer func() {
		tmpFile.Close()
		if !streamSuccess {
			os.Remove(writeCachePath)
		}
	}()

	pipeline := pipelines.NewEncodingPipeline(stream).
		EnableHashVerification(expectedHash)

	validStream, err := pipeline.Build()
	if err != nil {
		return fmt.Errorf("failed to build pipeline: %w", err)
	}

	if _, err := io.Copy(tmpFile, validStream); err != nil {
		return fmt.Errorf("upload or verification failed: %w", err)
	}

	tmpFile.Sync()
	tmpFile.Close()

	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fmt.Errorf("failed to create final dir: %w", err)
	}

	if err := os.Rename(writeCachePath, finalPath); err != nil {
		return fmt.Errorf("failed to finalize write: %w", err)
	}

	streamSuccess = true
	return l.writesRepo.DeleteUploadPath(ctx, writeId)
}
