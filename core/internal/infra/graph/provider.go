package graph

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type GlobalConfig struct {
	Backend string `env:"GRAPH_BACKEND" envDefault:"bolt"`
}

// NewFromEnv parses the environment and returns the configured Graph database.
func NewFromEnv() (Graph, error) {
	var cfg GlobalConfig
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse global graph config: %w", err)
	}

	switch cfg.Backend {
	case "bolt":
		return NewBoltGraph()
	default:
		return nil, fmt.Errorf("unsupported graph backend: %s", cfg.Backend)
	}
}
