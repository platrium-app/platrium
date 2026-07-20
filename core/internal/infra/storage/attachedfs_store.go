package storage

import (
	"context"
	"time"

	"platrium/internal/infra/kvstore"
)

// AttachedFSStore handles temporary upload sessions for the AttachedFS backend.
// Defining this interface makes unit testing and mocking significantly easier.
type AttachedFSStore interface {
	SetUploadPath(ctx context.Context, writeId string, path string) error
	GetUploadPath(ctx context.Context, writeId string) (string, error)
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

func (r *attachedFSStore) SetUploadPath(ctx context.Context, writeId string, path string) error {
	return r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
		key := kvstore.Key{Namespace: kvstore.NSAttachedFSWrites, ID: writeId}
		return tx.Set(key, []byte(path), kvstore.WithTTL(24*time.Hour))
	})
}

func (r *attachedFSStore) GetUploadPath(ctx context.Context, writeId string) (string, error) {
	var relPath string

	err := r.store.ReadTx(ctx, func(tx kvstore.Tx) error {
		key := kvstore.Key{Namespace: kvstore.NSAttachedFSWrites, ID: writeId}
		rawBytes, err := tx.Get(key)
		if err != nil {
			return err
		}
		relPath = string(rawBytes)
		return nil
	})

	return relPath, err
}

func (r *attachedFSStore) DeleteUploadPath(ctx context.Context, writeId string) error {
	return r.store.WriteTx(ctx, func(tx kvstore.Tx) error {
		key := kvstore.Key{Namespace: kvstore.NSAttachedFSWrites, ID: writeId}
		return tx.Delete(key)
	})
}
