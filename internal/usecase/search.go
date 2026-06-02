package usecase

import (
	"context"
	"sort"
	"strings"

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
	Query    string
	Tags     []string
	Notebook string
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

	limit := in.Limit * 2

	// Keyword search (always runs, doesn't depend on Ollama)
	ftsResults, _ := uc.repo.SearchByText(ctx, in.Query, in.Tags, in.Notebook, limit)

	// Vector search (best-effort, depends on Ollama)
	var vecResults []domain.ScoredDocument
	embedding, embErr := uc.embed.Generate(ctx, in.Query)
	if embErr == nil {
		vecResults, _ = uc.repo.SearchByEmbedding(ctx, embedding, in.Tags, in.Notebook, limit)
	}

	// Merge hybrid results
	merged := mergeHybrid(ftsResults, vecResults)

	// Sort by hybrid score descending
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	if len(merged) > in.Limit {
		merged = merged[:in.Limit]
	}

	results := make([]*SearchResult, len(merged))
	for i, r := range merged {
		results[i] = &SearchResult{
			Document: r.Document,
			Score:    r.Score,
			Snippet:  extractSnippet(r.Document.Content, in.Query),
		}
	}
	return results, nil
}

type scoredDoc struct {
	doc      *domain.Document
	ftsScore float32
	vecScore float32
}

func mergeHybrid(fts, vec []domain.ScoredDocument) []domain.ScoredDocument {
	scored := make(map[string]*scoredDoc)

	for _, r := range fts {
		id := r.Document.ID
		if _, ok := scored[id]; !ok {
			scored[id] = &scoredDoc{doc: r.Document}
		}
		scored[id].ftsScore = r.Score
	}

	for _, r := range vec {
		id := r.Document.ID
		if _, ok := scored[id]; !ok {
			scored[id] = &scoredDoc{doc: r.Document}
		}
		scored[id].vecScore = r.Score
	}

	var results []domain.ScoredDocument
	for _, s := range scored {
		var hybrid float32
		if s.vecScore > 0 && s.ftsScore > 0 {
			hybrid = s.ftsScore*0.35 + s.vecScore*0.65
		} else if s.vecScore > 0 {
			hybrid = s.vecScore
		} else {
			hybrid = s.ftsScore
		}
		results = append(results, domain.ScoredDocument{Document: s.doc, Score: hybrid})
	}
	return results
}

func extractSnippet(content, query string) string {
	paragraphs := strings.Split(content, "\n\n")
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	bestScore := 0
	bestPara := ""

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pLower := strings.ToLower(p)
		score := 0
		for _, w := range queryWords {
			if strings.Contains(pLower, w) {
				score += strings.Count(pLower, w)
			}
		}
		// Prefer earlier paragraphs
		if score > 0 {
			score += 100
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

	if len(bestPara) > 200 {
		idx := strings.Index(strings.ToLower(bestPara), queryLower)
		if idx < 0 {
			for _, w := range queryWords {
				idx = strings.Index(strings.ToLower(bestPara), w)
				if idx >= 0 {
					break
				}
			}
		}
		if idx > 0 {
			start := idx - 80
			if start < 0 {
				start = 0
			}
			end := start + 200
			if end > len(bestPara) {
				end = len(bestPara)
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
		return bestPara[:197] + "…"
	}

	return bestPara
}
