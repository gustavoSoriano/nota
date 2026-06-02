package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/soriano/nota/internal/domain"
)

type RestoreUseCase struct {
	repo domain.DocumentRepository
}

func NewRestoreUseCase(repo domain.DocumentRepository) *RestoreUseCase {
	return &RestoreUseCase{repo: repo}
}

func (uc *RestoreUseCase) Execute(ctx context.Context, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading backup: %w", err)
	}

	var backup []struct {
		ID        string   `json:"id"`
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		Tags      []string `json:"tags"`
		Notebook  string   `json:"notebook"`
		Category  string   `json:"category"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
		Accessed  int      `json:"accessed"`
	}
	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("parsing backup: %w", err)
	}

	docs := make([]*domain.Document, len(backup))
	for i, b := range backup {
		var createdAt, updatedAt time.Time
		if t, err := time.Parse(time.RFC3339, b.CreatedAt); err == nil {
			createdAt = t
		}
		if t, err := time.Parse(time.RFC3339, b.UpdatedAt); err == nil {
			updatedAt = t
		}
		docs[i] = &domain.Document{
			ID:        b.ID,
			Title:     b.Title,
			Content:   b.Content,
			Tags:      b.Tags,
			Notebook:  b.Notebook,
			Category:  b.Category,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			Accessed:  b.Accessed,
		}
	}

	return uc.repo.ImportMany(ctx, docs)
}
