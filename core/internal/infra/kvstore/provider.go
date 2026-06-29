package kvstore

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type GlobalConfig struct {
	Backend string `env:"KV_BACKEND" envDefault:"badger"`
}

// NewFromEnv parses the environment and returns the configured KVStore.
func NewFromEnv() (KVStore, error) {
	var cfg GlobalConfig
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse global kv config: %w", err)
	}

	switch cfg.Backend {
	case "badger":
		return NewBadgerStore()
	case "tikv":
		// return NewTiKVStore()
		return nil, fmt.Errorf("tikv adapter not yet implemented")
	default:
		return nil, fmt.Errorf("unknown kv backend: %s", cfg.Backend)
	}
}
