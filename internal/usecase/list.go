package usecase

import (
	"context"

	"github.com/soriano/nota/internal/domain"
)

type ListUseCase struct {
	repo domain.DocumentRepository
}

func NewListUseCase(repo domain.DocumentRepository) *ListUseCase {
	return &ListUseCase{repo: repo}
}

type ListInput struct {
	Tags      []string
	Grupo     string
	Categoria string
	Sort      string
	Limit     int
	Offset    int
}

func (uc *ListUseCase) Execute(ctx context.Context, in ListInput) ([]*domain.Document, error) {
	if in.Limit <= 0 {
		in.Limit = 20
	}
	return uc.repo.List(ctx, domain.ListFilter{
		Tags:      in.Tags,
		Grupo:     in.Grupo,
		Categoria: in.Categoria,
		Sort:      in.Sort,
		Limit:     in.Limit,
		Offset:    in.Offset,
	})
}
