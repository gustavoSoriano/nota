package sqlite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/soriano/nota/internal/domain"
)

type configRepo struct {
	path string
}

func NewConfigRepository(storagePath string) *configRepo {
	return &configRepo{path: filepath.Join(storagePath, "config.json")}
}

func (r *configRepo) Load(ctx context.Context) (*domain.Config, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, err
	}
	var cfg domain.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *configRepo) Save(ctx context.Context, cfg *domain.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0644)
}

func (r *configRepo) Exists(ctx context.Context) bool {
	_, err := os.Stat(r.path)
	return err == nil
}
