package kvstore

import (
	"context"

	"github.com/caarlos0/env/v11"
	"github.com/dgraph-io/badger/v4"
)

type BadgerConfig struct {
	Path string `env:"KV_BADGER_PATH,required"`
}

type BadgerStore struct {
	db *badger.DB
}

// Ensure BadgerStore implements KVStore
var _ KVStore = (*BadgerStore)(nil)

func NewBadgerStore() (*BadgerStore, error) {
	var cfg BadgerConfig
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	opt := badger.DefaultOptions(cfg.Path)
	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}

	return &BadgerStore{db: db}, nil
}

func (b *BadgerStore) Get(ctx context.Context, key string) ([]byte, error) {
	var valCopy []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		valCopy, err = item.ValueCopy(nil)
		return err
	})
	return valCopy, err
}

func (b *BadgerStore) Set(ctx context.Context, key string, value []byte, opts ...SetOption) error {
	options := &SetOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return b.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), value)
		if options.TTL > 0 {
			entry = entry.WithTTL(options.TTL)
		}
		return txn.SetEntry(entry)
	})
}

func (b *BadgerStore) Delete(ctx context.Context, key string) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Close gracefully shuts down the database. Should be called on application exit.
func (b *BadgerStore) Close() error {
	return b.db.Close()
}
