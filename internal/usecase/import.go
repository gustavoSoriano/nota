package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/soriano/nota/internal/domain"
)

type ImportUseCase struct {
	repo  domain.DocumentRepository
	embed domain.EmbeddingService
}

func NewImportUseCase(repo domain.DocumentRepository, embed domain.EmbeddingService) *ImportUseCase {
	return &ImportUseCase{repo: repo, embed: embed}
}

type ImportInput struct {
	Path  string
	Tags  []string
	Notebook string
}

func (uc *ImportUseCase) Execute(ctx context.Context, in ImportInput) ([]*domain.Document, error) {
	info, err := os.Stat(in.Path)
	if err != nil {
		return nil, fmt.Errorf("path not found: %w", err)
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(in.Path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
				files = append(files, filepath.Join(in.Path, e.Name()))
			}
		}
	} else {
		if filepath.Ext(in.Path) != ".md" {
			return nil, fmt.Errorf("only .md files are supported")
		}
		files = append(files, in.Path)
	}

	var imported []*domain.Document
	for _, f := range files {
		doc, err := uc.importFile(ctx, f, in)
		if err != nil {
			return imported, fmt.Errorf("importing %s: %w", f, err)
		}
		imported = append(imported, doc)
	}
	return imported, nil
}

func (uc *ImportUseCase) importFile(ctx context.Context, path string, in ImportInput) (*domain.Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime()
	doc := &domain.Document{
		ID:        uuid.New().String()[:8],
		Content:   string(content),
		Tags:      in.Tags,
		Notebook:  in.Notebook,
		CreatedAt: modTime,
		UpdatedAt: modTime,
	}
	doc.Title = doc.ExtractTitle()

	embedding, err := uc.embed.Generate(ctx, string(content))
	if err == nil {
		doc.Embedding = embedding
	}

	if err := uc.repo.Create(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}
