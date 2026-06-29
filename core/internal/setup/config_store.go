package setup

import (
	"context"

	"platrium/internal/infra/kvstore"
)

type ConfigKey string

const (
	KeyNativeTenantID ConfigKey = "native_tenant_id"
)

// InstanceConfigStore manages global deployment configuration in the KV Store.
type InstanceConfigStore interface {
	Get(ctx context.Context, key ConfigKey) (string, error)
	Set(ctx context.Context, key ConfigKey, value string) error
}

type instanceConfigStore struct {
	store kvstore.KVStore
}

// Ensure instanceConfigStore implements InstanceConfigStore
var _ InstanceConfigStore = (*instanceConfigStore)(nil)

func NewInstanceConfigStore(store kvstore.KVStore) InstanceConfigStore {
	return &instanceConfigStore{
		store: store,
	}
}

func (s *instanceConfigStore) Get(ctx context.Context, key ConfigKey) (string, error) {
	var val string
	err := s.store.ReadTx(ctx, func(tx kvstore.Tx) error {
		raw, err := tx.Get(kvstore.Key{Namespace: kvstore.NSInstanceConfig, ID: string(key)})
		if err != nil {
			return err
		}
		val = string(raw)
		return nil
	})
	return val, err
}

func (s *instanceConfigStore) Set(ctx context.Context, key ConfigKey, value string) error {
	return s.store.WriteTx(ctx, func(tx kvstore.Tx) error {
		return tx.Set(kvstore.Key{Namespace: kvstore.NSInstanceConfig, ID: string(key)}, []byte(value))
	})
}
