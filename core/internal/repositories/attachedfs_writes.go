package repositories

import (
	"context"
	"encoding/json"
	"time"

	"platrium/internal/infra/kvstore"
)

// WriteSession encapsulates the physical location and logical path for an upload session.
type WriteSession struct {
	Location string `json:"location"`
	Path     string `json:"path"`
}

// AttachedFSWritesRepository handles temporary upload sessions for the AttachedFS backend.
// Defining this interface makes unit testing and mocking significantly easier.
type AttachedFSWritesRepository interface {
	SetUploadPath(ctx context.Context, writeId string, session WriteSession) error
	GetUploadPath(ctx context.Context, writeId string) (WriteSession, error)
	DeleteUploadPath(ctx context.Context, writeId string) error
}

type AttachedFSWritesRepo struct {
	store kvstore.KVStore
}

// Ensure AttachedFSWritesRepo implements AttachedFSWritesRepository
var _ AttachedFSWritesRepository = (*AttachedFSWritesRepo)(nil)

func NewAttachedFSWritesRepository(store kvstore.KVStore) *AttachedFSWritesRepo {
	return &AttachedFSWritesRepo{
		store: store,
	}
}

func (r *AttachedFSWritesRepo) SetUploadPath(ctx context.Context, writeId string, session WriteSession) error {
	key := kvstore.BuildKey(NamespaceAttachedFSWrites, writeId)
	
	bytes, err := json.Marshal(session)
	if err != nil {
		return err
	}
	
	return r.store.Set(ctx, key, bytes, kvstore.WithTTL(24*time.Hour))
}

func (r *AttachedFSWritesRepo) GetUploadPath(ctx context.Context, writeId string) (WriteSession, error) {
	var session WriteSession
	key := kvstore.BuildKey(NamespaceAttachedFSWrites, writeId)
	rawBytes, err := r.store.Get(ctx, key)
	if err != nil {
		return session, err
	}

	err = json.Unmarshal(rawBytes, &session)
	return session, err
}

func (r *AttachedFSWritesRepo) DeleteUploadPath(ctx context.Context, writeId string) error {
	key := kvstore.BuildKey(NamespaceAttachedFSWrites, writeId)
	return r.store.Delete(ctx, key)
}
