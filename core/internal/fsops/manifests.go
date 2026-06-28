package fsops

import (
	"context"
	"fmt"

	"platrium/internal/infra/kvstore"
)

const (
	ManifestPageSize  = 1000
	NamespaceManifest = "mfst"
)

// ManifestRepo manages the storage of file chunk hashes in the KV Store.
type ManifestRepo struct {
	store kvstore.KVStore
}

// NewManifestRepo initializes a ManifestRepo.
func NewManifestRepo(store kvstore.KVStore) *ManifestRepo {
	return &ManifestRepo{store: store}
}

// SaveManifest chunks the binary hashes into pages of 32KB and writes them.
func (r *ManifestRepo) SaveManifest(ctx context.Context, fileId string, version int, hashes [][]byte) error {
	prefix := fmt.Sprintf("%s:%s:v%d", NamespaceManifest, fileId, version)

	/*
		// TODO: Add proper transaction wrappers to KVStore interface later
		return r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
			// ...
		})
	*/

	// Temporarily executing writes directly without a transaction so we can test the API
	total := len(hashes)
	if total == 0 {
		return r.store.Set(ctx, fmt.Sprintf("%s:0", prefix), []byte{})
	}

	page := 0
	for i := 0; i < total; i += ManifestPageSize {
		end := i + ManifestPageSize
		if end > total {
			end = total
		}

		var payload []byte
		for _, h := range hashes[i:end] {
			payload = append(payload, h...)
		}

		key := fmt.Sprintf("%s:%d", prefix, page)
		if err := r.store.Set(ctx, key, payload); err != nil {
			return err
		}
		page++
	}
	return nil
}
