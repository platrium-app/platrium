package fsops

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"platrium/internal/infra/kvstore"
)

type ChunkState byte

const (
	ChunkStateGCMarked  ChunkState = 'G'
	ChunkStateValidated ChunkState = 'V'
)

func (c ChunkState) String() string {
	switch c {
	case ChunkStateValidated:
		return "VALIDATED"
	case ChunkStateGCMarked:
		return "GC_MARKED"
	default:
		return "UNKNOWN"
	}
}

// ChunkMetadata represents the lifecycle and location of a chunk.
// Packed Binary Format (Zero-Reflection Wire Protocol):
// [0]     : ChunkState (1 byte: 'G' or 'V')
// [1:9]   : LastReferencedDay (8 bytes: Big-Endian int64)
// [9:]    : StorageBackend (Remaining bytes: raw string)
type ChunkMetadata struct {
	State             ChunkState
	StorageBackend    string
	LastReferencedDay int64
}

// Represents a Validated Chunk from the Storage Backend
// Struct is used to Add a new validated chunk to the store.
type ValidatedChunk struct {
	Hash      string
	BackendId string
}

// Marshal serializes the struct into a packed byte array without using reflection.
func (m *ChunkMetadata) Marshal() []byte {
	b := make([]byte, 1+8+len(m.StorageBackend))
	b[0] = byte(m.State)
	binary.BigEndian.PutUint64(b[1:9], uint64(m.LastReferencedDay))
	copy(b[9:], m.StorageBackend)
	return b
}

// UnmarshalDeserializes the packed byte array directly into the struct.
func UnmarshalChunkMetadata(b []byte) (*ChunkMetadata, error) {
	if len(b) < 9 {
		return nil, errors.New("malformed chunk metadata payload")
	}
	return &ChunkMetadata{
		State:             ChunkState(b[0]),
		LastReferencedDay: int64(binary.BigEndian.Uint64(b[1:9])),
		StorageBackend:    string(b[9:]),
	}, nil
}

type ChunkStore struct {
	store kvstore.KVStore
}

func NewChunkStore(store kvstore.KVStore) *ChunkStore {
	return &ChunkStore{store: store}
}

func todayTimestamp() int64 {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.Unix()
}

// GetChunks retrieves the metadata for the requested chunks via optimized point lookups.
func (r *ChunkStore) GetChunks(ctx context.Context, hashes []string) (map[string]*ChunkMetadata, error) {
	keys := make([]kvstore.Key, len(hashes))
	for i, hash := range hashes {
		keys[i] = kvstore.Key{Namespace: kvstore.NSChunkMetadata, ID: hash}
	}

	var results map[string][]byte
	err := r.store.ReadTx(ctx, func(tx kvstore.Tx) error {
		res, err := tx.MultiGet(keys)
		results = res
		return err
	})
	if err != nil {
		return nil, err
	}

	metaMap := make(map[string]*ChunkMetadata, len(results))
	for hash, data := range results {
		if meta, err := UnmarshalChunkMetadata(data); err == nil {
			metaMap[hash] = meta
		}
	}

	return metaMap, nil
}

// Checks existence of chunks and updates the last referenced timestamp for existing PRESENT chunks.
// Missing chunks are NOT written to the KV store (Stateless Presign). They are only written when storage confirms upload via MarkPresent.
func (r *ChunkStore) ReferenceExistingChunks(ctx context.Context, hashes []string) (map[string]*ChunkMetadata, error) {
	today := todayTimestamp()

	current, err := r.GetChunks(ctx, hashes)
	if err != nil {
		return nil, err
	}

	updates := make(map[kvstore.Key][]byte)

	for hash, meta := range current {
		if meta.State == ChunkStateValidated && meta.LastReferencedDay < today {
			meta.LastReferencedDay = today
			updates[kvstore.Key{Namespace: kvstore.NSChunkMetadata, ID: hash}] = meta.Marshal()
		}
	}

	if len(updates) > 0 {
		err = r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
			return tx.MultiSet(updates)
		})
		if err != nil {
			return nil, err
		}
	}

	return current, nil
}

// Creates a new chunk and marks it as VALIDATED.
func (r *ChunkStore) AddValidatedChunks(ctx context.Context, chunks []ValidatedChunk) error {
	today := todayTimestamp()
	updates := make(map[kvstore.Key][]byte, len(chunks))

	// Synthesize metadata states directly in memory from the cryptographic storage receipt
	for _, chunk := range chunks {
		meta := &ChunkMetadata{
			State:             ChunkStateValidated,
			StorageBackend:    chunk.BackendId,
			LastReferencedDay: today,
		}
		key := kvstore.Key{Namespace: kvstore.NSChunkMetadata, ID: chunk.Hash}
		updates[key] = meta.Marshal()
	}

	// Blast directly to the engine driver in a single forward write operation
	return r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
		return tx.MultiSet(updates)
	})
}
