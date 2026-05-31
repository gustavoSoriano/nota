package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/soriano/nota/internal/domain"
)

type linkRepo struct {
	db *sql.DB
}

func NewLinkRepository(db *sql.DB) *linkRepo {
	return &linkRepo{db: db}
}

func (r *linkRepo) Create(ctx context.Context, sourceID, targetID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO document_links (source_id, target_id) VALUES (?, ?)`,
		sourceID, targetID)
	return err
}

func (r *linkRepo) Delete(ctx context.Context, sourceID, targetID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM document_links WHERE source_id = ? AND target_id = ?`,
		sourceID, targetID)
	return err
}

func (r *linkRepo) GetLinked(ctx context.Context, docID string) ([]*domain.Document, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT d.id, d.title, d.content, d.tags, d.grupo, d.categoria, d.embedding, d.created_at, d.updated_at, d.accessed
		 FROM documents d
		 JOIN document_links l ON l.target_id = d.id
		 WHERE l.source_id = ?
		 UNION
		 SELECT d.id, d.title, d.content, d.tags, d.grupo, d.categoria, d.embedding, d.created_at, d.updated_at, d.accessed
		 FROM documents d
		 JOIN document_links l ON l.source_id = d.id
		 WHERE l.target_id = ?`, docID, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*domain.Document
	for rows.Next() {
		var doc domain.Document
		var tagsJSON string
		var embedBlob []byte
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &tagsJSON, &doc.Grupo, &doc.Categoria,
			&embedBlob, &doc.CreatedAt, &doc.UpdatedAt, &doc.Accessed); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsJSON), &doc.Tags)
		doc.Embedding = blobToFloat32(embedBlob)
		docs = append(docs, &doc)
	}
	return docs, nil
}
