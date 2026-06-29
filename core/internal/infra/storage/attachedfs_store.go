package storage

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

// AttachedFSStore handles temporary upload sessions for the AttachedFS backend.
// Defining this interface makes unit testing and mocking significantly easier.
type AttachedFSStore interface {
	SetUploadPath(ctx context.Context, writeId string, session WriteSession) error
	GetUploadPath(ctx context.Context, writeId string) (*WriteSession, error)
	DeleteUploadPath(ctx context.Context, writeId string) error
}

type attachedFSStore struct {
	store kvstore.KVStore
}

func NewAttachedFSStore(store kvstore.KVStore) AttachedFSStore {
	return &attachedFSStore{
		store: store,
	}
}

func (r *attachedFSStore) SetUploadPath(ctx context.Context, writeId string, session WriteSession) error {
	bytes, err := json.Marshal(session)
	if err != nil {
		return err
	}
	
	return r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
		key := kvstore.Key{Namespace: kvstore.NSAttachedFSWrites, ID: writeId}
		return tx.Set(key, bytes, kvstore.WithTTL(24*time.Hour))
	})
}

func (r *attachedFSStore) GetUploadPath(ctx context.Context, writeId string) (*WriteSession, error) {
	var session WriteSession
	
	err := r.store.ReadTx(ctx, func(tx kvstore.Tx) error {
		key := kvstore.Key{Namespace: kvstore.NSAttachedFSWrites, ID: writeId}
		rawBytes, err := tx.Get(key)
		if err != nil {
			return err
		}
		return json.Unmarshal(rawBytes, &session)
	})
	
	return &session, err
}

func (r *attachedFSStore) DeleteUploadPath(ctx context.Context, writeId string) error {
	return r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
		key := kvstore.Key{Namespace: kvstore.NSAttachedFSWrites, ID: writeId}
		return tx.Delete(key)
	})
}
