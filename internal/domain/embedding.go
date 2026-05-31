package domain

import "context"

type EmbeddingService interface {
	Generate(ctx context.Context, text string) ([]float32, error)
	GenerateBatch(ctx context.Context, texts []string) ([][]float32, error)
	CheckAvailability(ctx context.Context) error
}
