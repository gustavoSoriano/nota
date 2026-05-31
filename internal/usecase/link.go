package usecase

import (
	"context"

	"github.com/soriano/nota/internal/domain"
)

type LinkUseCase struct {
	docRepo  domain.DocumentRepository
	linkRepo domain.DocumentLinkRepository
}

func NewLinkUseCase(docRepo domain.DocumentRepository, linkRepo domain.DocumentLinkRepository) *LinkUseCase {
	return &LinkUseCase{docRepo: docRepo, linkRepo: linkRepo}
}

func (uc *LinkUseCase) Execute(ctx context.Context, sourceID, targetID string) error {
	if _, err := uc.docRepo.GetByID(ctx, sourceID); err != nil {
		return err
	}
	if _, err := uc.docRepo.GetByID(ctx, targetID); err != nil {
		return err
	}
	return uc.linkRepo.Create(ctx, sourceID, targetID)
}

func (uc *LinkUseCase) GetLinked(ctx context.Context, docID string) ([]*domain.Document, error) {
	return uc.linkRepo.GetLinked(ctx, docID)
}
