package usecase

import (
	"context"
	"fmt"

	"github.com/soriano/nota/internal/domain"
)

type DeleteUseCase struct {
	repo domain.DocumentRepository
}

func NewDeleteUseCase(repo domain.DocumentRepository) *DeleteUseCase {
	return &DeleteUseCase{repo: repo}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, id string) error {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		return fmt.Errorf("document not found: %w", err)
	}
	return uc.repo.Delete(ctx, id)
}
