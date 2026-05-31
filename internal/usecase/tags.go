package usecase

import (
	"context"

	"github.com/soriano/nota/internal/domain"
)

type TagsUseCase struct {
	repo domain.DocumentRepository
}

func NewTagsUseCase(repo domain.DocumentRepository) *TagsUseCase {
	return &TagsUseCase{repo: repo}
}

func (uc *TagsUseCase) Execute(ctx context.Context) (map[string]int, error) {
	return uc.repo.GetAllTags(ctx)
}
