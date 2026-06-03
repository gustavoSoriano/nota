package usecase

import (
	"context"
	"strings"

	"github.com/soriano/nota/internal/domain"
)

type SearchUseCase struct {
	repo domain.DocumentRepository
}

func NewSearchUseCase(repo domain.DocumentRepository) *SearchUseCase {
	return &SearchUseCase{repo: repo}
}

type SearchInput struct {
	Query    string
	Tags     []string
	Notebook string
	Category string
	Limit    int
}

type SearchResult struct {
	Document *domain.Document
	Score    float32
	Snippet  string
}

func (uc *SearchUseCase) Execute(ctx context.Context, in SearchInput) ([]*SearchResult, error) {
	if in.Limit <= 0 {
		in.Limit = 40
	}

	scored, err := uc.repo.SearchByText(ctx, in.Query, in.Tags, in.Notebook, in.Category, in.Limit)
	if err != nil {
		return nil, err
	}

	results := make([]*SearchResult, len(scored))
	for i, r := range scored {
		results[i] = &SearchResult{
			Document: r.Document,
			Score:    r.Score,
			Snippet:  extractSnippet(r.Document.Content, in.Query),
		}
	}
	return results, nil
}

func extractSnippet(content, query string) string {
	paragraphs := strings.Split(content, "\n\n")
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	bestScore := -1
	bestPara := ""

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pLower := strings.ToLower(p)
		score := 0
		for _, w := range queryWords {
			score += strings.Count(pLower, w)
		}
		if score > bestScore {
			bestScore = score
			bestPara = p
		}
	}

	if bestPara == "" && len(paragraphs) > 0 {
		bestPara = strings.TrimSpace(paragraphs[0])
	}
	if bestPara == "" {
		return ""
	}

	if len(bestPara) <= 200 {
		return bestPara
	}

	// Centralizar o snippet ao redor do primeiro match
	idx := -1
	for _, w := range queryWords {
		i := strings.Index(strings.ToLower(bestPara), w)
		if i >= 0 && (idx < 0 || i < idx) {
			idx = i
		}
	}

	if idx < 0 {
		return bestPara[:197] + "…"
	}

	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := start + 200
	if end > len(bestPara) {
		end = len(bestPara)
		start = end - 200
		if start < 0 {
			start = 0
		}
	}

	snippet := bestPara[start:end]
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(bestPara) {
		snippet = snippet + "…"
	}
	return snippet
}
