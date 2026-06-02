package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/soriano/nota/internal/domain"
)

type BackupUseCase struct {
	repo       domain.DocumentRepository
	storageDir string
}

func NewBackupUseCase(repo domain.DocumentRepository, storageDir string) *BackupUseCase {
	return &BackupUseCase{repo: repo, storageDir: storageDir}
}

func (uc *BackupUseCase) Execute(ctx context.Context) (string, error) {
	docs, err := uc.repo.ExportAll(ctx)
	if err != nil {
		return "", fmt.Errorf("exporting: %w", err)
	}

	backup := make([]map[string]any, len(docs))
	for i, d := range docs {
		backup[i] = map[string]any{
			"id":         d.ID,
			"title":      d.Title,
			"content":    d.Content,
			"tags":       d.Tags,
			"notebook":   d.Notebook,
			"category":   d.Category,
			"created_at": d.CreatedAt,
			"updated_at": d.UpdatedAt,
			"accessed":   d.Accessed,
		}
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", err
	}

	backupDir := filepath.Join(uc.storageDir, "backups")
	os.MkdirAll(backupDir, 0755)

	name := fmt.Sprintf("nota-backup-%s.json", time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(backupDir, name)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}
