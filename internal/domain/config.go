package domain

import "context"

type Config struct {
	Editor      string `json:"editor"`
	StoragePath string `json:"storage_path"`
}

type ConfigRepository interface {
	Load(ctx context.Context) (*Config, error)
	Save(ctx context.Context, cfg *Config) error
	Exists(ctx context.Context) bool
}
