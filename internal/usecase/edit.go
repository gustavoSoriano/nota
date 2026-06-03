package usecase

import (
	"context"
	"fmt"

	"github.com/soriano/nota/internal/domain"
	"github.com/soriano/nota/internal/infra/editor"
)

type EditUseCase struct {
	repo   domain.DocumentRepository
	editor string
}

func NewEditUseCase(repo domain.DocumentRepository, editor string) *EditUseCase {
	return &EditUseCase{repo: repo, editor: editor}
}

func (uc *EditUseCase) Execute(ctx context.Context, id string) error {
	doc, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	tmp, err := editor.CreateTempFile(doc.Content)
	if err != nil {
		return err
	}
	defer editor.Cleanup(tmp)

	if err := editor.Open(uc.editor, tmp); err != nil {
		return err
	}

	content, err := editor.ReadFile(tmp)
	if err != nil {
		return err
	}
	if content == "" {
		return fmt.Errorf("empty content, aborting")
	}

	doc.Content = content
	doc.Title = doc.ExtractTitle()

	return uc.repo.Update(ctx, doc)
}
