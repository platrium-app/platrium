package fsops

import (
	"context"
	"fmt"

	"platrium/internal/infra/kvstore"
)

const (
	ManifestPageSize  = 1000
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
	prefix := fmt.Sprintf("%s:v%d", fileId, version)

	return r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
		total := len(hashes)
		if total == 0 {
			// Write empty page 0
			return tx.Set(kvstore.Key{Namespace: kvstore.NSManifest, ID: fmt.Sprintf("%s:0", prefix)}, []byte{})
		}

		kvs := make(map[kvstore.Key][]byte)
		page := 0
		for i := 0; i < total; i += ManifestPageSize {
			end := min(i+ManifestPageSize, total)

			// Flatten the slice of 32-byte hashes into a contiguous byte slice
			var payload []byte
			for _, h := range hashes[i:end] {
				payload = append(payload, h...)
			}

			key := kvstore.Key{Namespace: kvstore.NSManifest, ID: fmt.Sprintf("%s:%d", prefix, page)}
			kvs[key] = payload
			page++
		}

		return tx.MultiSet(kvs)
	})
}
