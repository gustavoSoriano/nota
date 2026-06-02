package usecase

import (
	"context"
	"math"

	"github.com/soriano/nota/internal/domain"
)

type SearchUseCase struct {
	repo  domain.DocumentRepository
	embed domain.EmbeddingService
}

func NewSearchUseCase(repo domain.DocumentRepository, embed domain.EmbeddingService) *SearchUseCase {
	return &SearchUseCase{repo: repo, embed: embed}
}

type SearchInput struct {
	Query     string
	Tags      []string
	Notebook   string
	Limit     int
}

type SearchResult struct {
	Document *domain.Document
	Score    float32
}

func (uc *SearchUseCase) Execute(ctx context.Context, in SearchInput) ([]*SearchResult, error) {
	if in.Limit <= 0 {
		in.Limit = 40
	}

	embedding, err := uc.embed.Generate(ctx, in.Query)
	if err != nil {
		return nil, err
	}

	docs, err := uc.repo.SearchByEmbedding(ctx, embedding, in.Tags, in.Notebook, in.Limit)
	if err != nil {
		return nil, err
	}

	results := make([]*SearchResult, len(docs))
	for i, doc := range docs {
		var score float32
		if len(doc.Embedding) > 0 && len(embedding) > 0 {
			score = cosineSim(embedding, doc.Embedding)
		}
		results[i] = &SearchResult{Document: doc, Score: score}
	}
	return results, nil
}

func cosineSim(a, b []float32) float32 {
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}
