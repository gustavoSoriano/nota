package usecase

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/soriano/nota/internal/domain"
)

type SaveUseCase struct {
	repo  domain.DocumentRepository
	embed domain.EmbeddingService
}

func NewSaveUseCase(repo domain.DocumentRepository, embed domain.EmbeddingService) *SaveUseCase {
	return &SaveUseCase{repo: repo, embed: embed}
}

type SaveInput struct {
	Content   string
	Tags      []string
	Notebook  string
	Category  string
	FromPipe  bool
}

func (uc *SaveUseCase) Execute(ctx context.Context, in SaveInput) (*domain.Document, error) {
	content := in.Content
	if in.FromPipe {
		stat, _ := os.Stdin.Stat()
		if stat.Mode()&os.ModeCharDevice == 0 {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("reading pipe: %w", err)
			}
			content = string(b)
		}
	}
	if content == "" {
		return nil, fmt.Errorf("empty content")
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
