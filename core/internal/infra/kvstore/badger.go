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

func (b *BadgerStore) ReadTx(ctx context.Context, fn func(tx Tx) error) error {
	return b.db.View(func(txn *badger.Txn) error {
		return fn(&BadgerTx{txn: txn})
	})
}

type BadgerTx struct {
	txn *badger.Txn
}

var _ Tx = (*BadgerTx)(nil)

func (t *BadgerTx) Get(key Key) ([]byte, error) {
	item, err := t.txn.Get([]byte(key.String()))
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

func (t *BadgerTx) MultiGet(keys []Key) (map[string][]byte, error) {
	res := make(map[string][]byte, len(keys))
	for _, k := range keys {
		val, err := t.Get(k)
		if err == badger.ErrKeyNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		res[k.ID] = val // Return the ID without the namespace to the caller
	}
	return res, nil
}

func (t *BadgerTx) Set(key Key, value []byte, opts ...SetOption) error {
	options := &SetOptions{}
	for _, opt := range opts {
		opt(options)
	}

	finalKey := []byte(key.String())
	if options.TTL > 0 {
		e := badger.NewEntry(finalKey, value).WithTTL(options.TTL)
		return t.txn.SetEntry(e)
	}
	return t.txn.Set(finalKey, value)
}

func (t *BadgerTx) MultiSet(kvs map[Key][]byte, opts ...SetOption) error {
	for k, v := range kvs {
		if err := t.Set(k, v, opts...); err != nil {
			return err
		}
	}
	return nil
}

func (t *BadgerTx) Delete(key Key) error {
	return t.txn.Delete([]byte(key.String()))
}

func (t *BadgerTx) MultiDelete(keys []Key) error {
	for _, k := range keys {
		if err := t.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

func (b *BadgerStore) WriteTx(ctx context.Context, fn func(tx Tx) error) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return fn(&BadgerTx{txn: txn})
	})
}

// Close gracefully shuts down the database. Should be called on application exit.
func (b *BadgerStore) Close() error {
	return b.db.Close()
}
