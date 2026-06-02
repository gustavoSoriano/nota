package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/soriano/nota/internal/domain"
	"github.com/soriano/nota/internal/infra/editor"
)

type CreateUseCase struct {
	repo    domain.DocumentRepository
	embed   domain.EmbeddingService
	editor  string
}

func NewCreateUseCase(repo domain.DocumentRepository, embed domain.EmbeddingService, editor string) *CreateUseCase {
	return &CreateUseCase{repo: repo, embed: embed, editor: editor}
}

type CreateInput struct {
	Tags      []string
	Notebook  string
	Category  string
	Content   string
}

func (uc *CreateUseCase) Execute(ctx context.Context, in CreateInput) (*domain.Document, error) {
	var content string
	if in.Content != "" {
		content = in.Content
	} else {
		tmp, err := editor.CreateTempFile("")
		if err != nil {
			return nil, fmt.Errorf("creating temp file: %w", err)
		}
		defer editor.Cleanup(tmp)

		if err := editor.Open(uc.editor, tmp); err != nil {
			return nil, fmt.Errorf("editor failed: %w", err)
		}
		content, err = editor.ReadFile(tmp)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}
		if content == "" {
			return nil, fmt.Errorf("empty document, aborting")
		}
	}

	now := time.Now()
	doc := &domain.Document{
		ID:        uuid.New().String()[:8],
		Content:   content,
		Tags:      in.Tags,
		Notebook:  in.Notebook,
		Category:  in.Category,
		CreatedAt: now,
		UpdatedAt: now,
	}
	doc.Title = doc.ExtractTitle()

	embedding, err := uc.embed.Generate(ctx, content)
	if err == nil {
		doc.Embedding = embedding
	}

	if err := uc.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("saving document: %w", err)
	}
	return doc, nil
}
