package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"

	"platrium/internal/fsops"
	"platrium/internal/pipelines"
)

// AttachedFSEnv reads global infrastructure details from the host server environment.
type AttachedFSEnv struct {
	AppURL     string `env:"APP_URL"`
	HMACSecret string `env:"HMAC_SECRET" envDefault:"changeinprod"`
}

// AttachedFSConfig holds the unique database parameters per isolated volume layout.
type AttachedFSConfig struct {
	MountPath string `json:"mount_path"` // e.g., "/mnt/nvme-drive-0" or "./data"
}

const WriteCacheDir = "writecache"

// AttachedFSBackend implements StorageBackend for any OS-attached file system.
type AttachedFSBackend struct {
	store        AttachedFSStore
	backendId    string
	mountPath    string
	apiBaseURL   string
	hmacSecret   string
	validationCh chan<- fsops.ValidatedChunk // Pure, raw Go channel. Completely thread-safe because it's immutable.
}

// Ensure the driver perfectly satisfies the core storage data contract
var _ Backend = (*AttachedFSBackend)(nil)

// AttachedFSBackendFactory handles the initial closure generation.
// It reads the global host setup once and dynamically parses instance parameters at boot.
func AttachedFSBackendFactory(store AttachedFSStore) BackendFactory {
	var envCfg AttachedFSEnv
	if err := env.Parse(&envCfg); err != nil {
		panic(fmt.Sprintf("failed to parse global AFS environment setup: %v", err))
	}

	return func(ctx context.Context, backendId string, rawConfig json.RawMessage) (Backend, error) {
		var instanceCfg AttachedFSConfig
		if err := json.Unmarshal(rawConfig, &instanceCfg); err != nil {
			return nil, fmt.Errorf("failed to parse AFS instance config: %w", err)
		}

		if instanceCfg.MountPath == "" {
			return nil, fmt.Errorf("mount_path is required for independent AFS instances")
		}

		return &AttachedFSBackend{
			store:      store,
			backendId:  backendId,
			mountPath:  instanceCfg.MountPath,
			apiBaseURL: envCfg.AppURL,
			hmacSecret: envCfg.HMACSecret,
		}, nil
	}
}

// SubscribeUploadEvents wires up the validation pipeline channel synchronously.
// Because the manager executes this BEFORE publishing the instance, it is 100% thread-safe.
func (l *AttachedFSBackend) SubscribeUploadEvents(ctx context.Context, chunkValidationCh chan<- fsops.ValidatedChunk) {
	l.validationCh = chunkValidationCh
}

// GenerateChunkUploadURLs registers temporary write sessions in KV and targets the local instance route with HMAC.
func (l *AttachedFSBackend) GenerateChunkUploadURLs(ctx context.Context, chunks map[string]ChunkUploadInfo) (map[string]string, error) {
	urls := make(map[string]string)
	for hash, info := range chunks {
		writeId := uuid.New().String()

		if err := l.store.SetUploadPath(ctx, writeId, info.Path); err != nil {
			return nil, fmt.Errorf("failed to create upload session: %w", err)
		}

		sig := l.SignWriteURL(writeId)
		urls[hash] = fmt.Sprintf("%s/api/attachedfs/%s/%s?sig=%s", l.apiBaseURL, l.backendId, writeId, sig)
	}
	return urls, nil
}

// GenerateChunkDownloadURLs explicitly hits the brakes and throws an unimplemented error for now.
func (l *AttachedFSBackend) GenerateChunkDownloadURLs(ctx context.Context, chunkHashes []string) (map[string]string, error) {
	return nil, fmt.Errorf("direct chunk download URLs are unimplemented for the attached filesystem backend")
}

// CommitLocalWrite safely streams the HTTP request body directly to disk, performing real-time cryptographic hash validation.
func (l *AttachedFSBackend) CommitLocalWrite(ctx context.Context, writeId string, signature string, stream io.Reader) error {
	// 0. Validate HMAC Signature
	if !l.VerifyWriteURL(writeId, signature) {
		return fmt.Errorf("Invalid Write Signature!")
	}

	// 1. Fetch active session relative path from the KV store
	relPath, err := l.store.GetUploadPath(ctx, writeId)
	if err != nil {
		return fmt.Errorf("invalid or expired write session: %w", err)
	}

	expectedHash := filepath.Base(relPath)
	writeCachePath := filepath.Join(l.mountPath, WriteCacheDir, writeId)
	finalPath := filepath.Join(l.mountPath, relPath)

	// 2. Provision directories for the staging write cache
	if err := os.MkdirAll(filepath.Dir(writeCachePath), 0755); err != nil {
		return fmt.Errorf("failed to create write cache dir: %w", err)
	}

	// 3. Create the temporary file inside the cache staging area
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

	// 4. Build the ingestion pipeline with inline cryptographic hash verification
	pipeline := pipelines.NewEncodingPipeline(stream).EnableHashVerification(expectedHash)
	validStream, err := pipeline.Build()
	if err != nil {
		return fmt.Errorf("failed to build pipeline: %w", err)
	}

	// 5. Stream the verified payload directly to the staging file
	if _, err := io.Copy(tmpFile, validStream); err != nil {
		return fmt.Errorf("upload or verification failed: %w", err)
	}

	// Flush writes to hardware platters before closing handles
	tmpFile.Sync()
	tmpFile.Close()

	// 6. Ensure target sharded chunk directories exist
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fmt.Errorf("failed to create final dir: %w", err)
	}

	// 7. Perform an exclusive finalize using a hard-link rotation.
	// Since writecache and chunks share the same volume mount, a hard link is instant and atomic.
	// If it already exists, Link fails with an 'exists' error, letting us drop the duplicate safely.
	err = os.Link(writeCachePath, finalPath)
	if err != nil {
		if os.IsExist(err) {
			// Duplicate chunk already successfully written by a concurrent worker. Drop ours cleanly.
			os.Remove(writeCachePath)
		} else {
			return fmt.Errorf("failed to link finalized chunk: %w", err)
		}
	} else {
		// Link succeeded; clean up the original staging cache pointer file
		os.Remove(writeCachePath)
	}

	streamSuccess = true

	// 8. Delete the temporary upload session metadata from the store
	if deleteErr := l.store.DeleteUploadPath(ctx, writeId); deleteErr != nil {
		return fmt.Errorf("failed to clear upload session keys: %w", deleteErr)
	}

	// 9. Hand off hash to the manager's validation queue. Blocks if the buffer is full.
	if l.validationCh != nil {
		select {
		case l.validationCh <- fsops.ValidatedChunk{
			Hash:      expectedHash,
			BackendId: l.backendId,
		}:

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// SignWriteURL generates a HMAC signature for a writeId bound to this backend instance.
func (l *AttachedFSBackend) SignWriteURL(writeId string) string {
	h := hmac.New(sha256.New, []byte(l.hmacSecret))
	h.Write([]byte(l.backendId + ":" + writeId))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyWriteURL checks if a provided signature matches the writeId bound to this backend instance.
func (l *AttachedFSBackend) VerifyWriteURL(writeId string, signature string) bool {
	expected := l.SignWriteURL(writeId)
	return hmac.Equal([]byte(expected), []byte(signature))
}
