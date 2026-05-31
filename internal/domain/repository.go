package domain

import "context"

type DocumentRepository interface {
	Create(ctx context.Context, doc *Document) error
	GetByID(ctx context.Context, id string) (*Document, error)
	Update(ctx context.Context, doc *Document) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ListFilter) ([]*Document, error)
	SearchByEmbedding(ctx context.Context, embedding []float32, tags []string, grupo string, limit int) ([]*Document, error)
	IncrementAccessed(ctx context.Context, id string) error
	GetAllTags(ctx context.Context) (map[string]int, error)
	DeleteAll(ctx context.Context) error
	ExportAll(ctx context.Context) ([]*Document, error)
	ImportMany(ctx context.Context, docs []*Document) error
}

type DocumentLinkRepository interface {
	Create(ctx context.Context, sourceID, targetID string) error
	Delete(ctx context.Context, sourceID, targetID string) error
	GetLinked(ctx context.Context, docID string) ([]*Document, error)
}

type ListFilter struct {
	Tags      []string
	Grupo     string
	Categoria string
	Sort      string
	Limit     int
	Offset    int
}
