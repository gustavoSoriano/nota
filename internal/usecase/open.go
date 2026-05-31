package usecase

import (
	"context"
	"fmt"

	"github.com/soriano/nota/internal/domain"
)

type OpenUseCase struct {
	repo domain.DocumentRepository
}

func NewOpenUseCase(repo domain.DocumentRepository) *OpenUseCase {
	return &OpenUseCase{repo: repo}
}

func (uc *OpenUseCase) Execute(ctx context.Context, id string) (*domain.Document, error) {
	doc, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}
	_ = uc.repo.IncrementAccessed(ctx, id)
	return doc, nil
}
