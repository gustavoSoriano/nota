package usecase

import (
	"context"

	"github.com/soriano/nota/internal/domain"
)

type CleanUseCase struct {
	repo domain.DocumentRepository
}

func NewCleanUseCase(repo domain.DocumentRepository) *CleanUseCase {
	return &CleanUseCase{repo: repo}
}

func (uc *CleanUseCase) Execute(ctx context.Context) error {
	return uc.repo.DeleteAll(ctx)
}
