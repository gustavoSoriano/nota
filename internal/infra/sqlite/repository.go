package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/soriano/nota/internal/domain"
)

type DocumentRepo struct {
	db     *sql.DB
	useFTS bool
}

func NewDocumentRepository(dbPath string) (*DocumentRepo, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating storage dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	r := &DocumentRepo{db: db}
	if err := r.migrate(); err != nil {
		return nil, fmt.Errorf("migrating: %w", err)
	}
	return r, nil
}

func (r *DocumentRepo) migrate() error {
	_, err := r.db.Exec(`
		PRAGMA journal_mode=WAL;
		CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT NOT NULL DEFAULT '[]',
			grupo TEXT NOT NULL DEFAULT '',
			categoria TEXT NOT NULL DEFAULT '',
			embedding BLOB,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			accessed INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS document_links (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			PRIMARY KEY (source_id, target_id),
			FOREIGN KEY (source_id) REFERENCES documents(id) ON DELETE CASCADE,
			FOREIGN KEY (target_id) REFERENCES documents(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_docs_grupo ON documents(grupo);
		CREATE INDEX IF NOT EXISTS idx_docs_categoria ON documents(categoria);
		CREATE INDEX IF NOT EXISTS idx_docs_created ON documents(created_at DESC);
	`)
	if err != nil {
		return err
	}

	_, ftsErr := r.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
			doc_id UNINDEXED,
			title,
			content,
			tokenize='unicode61'
		)
	`)
	if ftsErr != nil {
		log.Printf("[nota] FTS5 unavailable, using LIKE fallback: %v", ftsErr)
		r.useFTS = false
	} else {
		r.useFTS = true
		r.rebuildFTS(context.Background())
	}
	return nil
}

func (r *DocumentRepo) syncFTS(ctx context.Context, docID, title, content string) {
	if !r.useFTS {
		return
	}
	r.db.ExecContext(ctx, `DELETE FROM docs_fts WHERE doc_id = ?`, docID)
	r.db.ExecContext(ctx, `INSERT INTO docs_fts (doc_id, title, content) VALUES (?, ?, ?)`, docID, title, content)
}

func (r *DocumentRepo) deleteFTS(ctx context.Context, docID string) {
	if !r.useFTS {
		return
	}
	r.db.ExecContext(ctx, `DELETE FROM docs_fts WHERE doc_id = ?`, docID)
}

func (r *DocumentRepo) rebuildFTS(ctx context.Context) {
	if !r.useFTS {
		return
	}
	r.db.ExecContext(ctx, `DELETE FROM docs_fts`)
	r.db.ExecContext(ctx, `INSERT INTO docs_fts (doc_id, title, content) SELECT id, title, content FROM documents`)
}

func (r *DocumentRepo) Create(ctx context.Context, doc *domain.Document) error {
	tagsJSON, _ := json.Marshal(doc.Tags)
	embedBlob := float32ToBlob(doc.Embedding)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO documents (id, title, content, tags, grupo, categoria, embedding, created_at, updated_at, accessed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Title, doc.Content, string(tagsJSON), doc.Notebook, doc.Category, embedBlob,
		doc.CreatedAt, doc.UpdatedAt, doc.Accessed,
	)
	if err == nil {
		r.syncFTS(ctx, doc.ID, doc.Title, doc.Content)
	}
	return err
}

func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*domain.Document, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, title, content, tags, grupo, categoria, embedding, created_at, updated_at, accessed
		 FROM documents WHERE id = ?`, id)
	return r.scanDocument(row)
}

func (r *DocumentRepo) Update(ctx context.Context, doc *domain.Document) error {
	tagsJSON, _ := json.Marshal(doc.Tags)
	embedBlob := float32ToBlob(doc.Embedding)
	_, err := r.db.ExecContext(ctx,
		`UPDATE documents SET title=?, content=?, tags=?, grupo=?, categoria=?, embedding=?, updated_at=?, accessed=?
		 WHERE id=?`,
doc.Title, doc.Content, string(tagsJSON), doc.Notebook, doc.Category, embedBlob,
		doc.UpdatedAt, doc.Accessed, doc.ID,
	)
	if err == nil {
		r.syncFTS(ctx, doc.ID, doc.Title, doc.Content)
	}
	return err
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	if err == nil {
		r.deleteFTS(ctx, id)
	}
	return err
}

func (r *DocumentRepo) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Document, error) {
	qry := `SELECT id, title, content, tags, grupo, categoria, embedding, created_at, updated_at, accessed
			FROM documents`
	where, args := r.buildWhere(filter)
	if where != "" {
		qry += " WHERE " + where
	}
	qry += " ORDER BY " + r.sortClause(filter.Sort)
	if filter.Limit <= 0 {
		filter.Limit = 40
	}
	qry += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, qry, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanDocuments(rows)
}

func (r *DocumentRepo) SearchByEmbedding(ctx context.Context, embedding []float32, tags []string, notebook string, limit int) ([]domain.ScoredDocument, error) {
	if limit <= 0 {
		limit = 40
	}
	qry := `SELECT id, title, content, tags, grupo, categoria, embedding, created_at, updated_at, accessed
			FROM documents`
	var conds []string
	var args []any
	if len(tags) > 0 {
		for _, t := range tags {
			conds = append(conds, "tags LIKE ?")
			args = append(args, `%"`+t+`"%`)
		}
	}
	if notebook != "" {
		conds = append(conds, "grupo = ?")
		args = append(args, notebook)
	}
	if len(conds) > 0 {
		qry += " WHERE " + strings.Join(conds, " AND ")
	}
	qry += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, qry, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs, err := r.scanDocuments(rows)
	if err != nil {
		return nil, err
	}

	var results []domain.ScoredDocument
	for _, d := range docs {
		if len(d.Embedding) == 0 {
			continue
		}
		s := cosineSimilarity(embedding, d.Embedding)
		results = append(results, domain.ScoredDocument{Document: d, Score: s})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (r *DocumentRepo) SearchByText(ctx context.Context, query string, tags []string, notebook string, limit int) ([]domain.ScoredDocument, error) {
	if limit <= 0 {
		limit = 40
	}
	if r.useFTS {
		return r.searchByTextFTS(ctx, query, tags, notebook, limit)
	}
	return r.searchByTextLike(ctx, query, tags, notebook, limit)
}

func (r *DocumentRepo) searchByTextFTS(ctx context.Context, query string, tags []string, notebook string, limit int) ([]domain.ScoredDocument, error) {
	ftsQuery := toFTSQuery(query)
	if ftsQuery == "" {
		return r.searchByTextLike(ctx, query, tags, notebook, limit)
	}

	qry := `
		SELECT d.id, d.title, d.content, d.tags, d.grupo, d.categoria, d.embedding, d.created_at, d.updated_at, d.accessed,
			   rank
		FROM docs_fts
		JOIN documents d ON d.id = docs_fts.doc_id
		WHERE docs_fts MATCH ?
	`
	var conds []string
	var args []any
	args = append(args, ftsQuery)

	if len(tags) > 0 {
		for _, t := range tags {
			conds = append(conds, "d.tags LIKE ?")
			args = append(args, `%"`+t+`"%`)
		}
	}
	if notebook != "" {
		conds = append(conds, "d.grupo = ?")
		args = append(args, notebook)
	}
	if len(conds) > 0 {
		qry += " AND " + strings.Join(conds, " AND ")
	}

	qry += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, qry, args...)
	if err != nil {
		return r.searchByTextLike(ctx, query, tags, notebook, limit)
	}
	defer rows.Close()

	var results []domain.ScoredDocument
	for rows.Next() {
		var doc domain.Document
		var tagsJSON string
		var embedBlob []byte
		var rank float64
		err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &tagsJSON, &doc.Notebook, &doc.Category,
			&embedBlob, &doc.CreatedAt, &doc.UpdatedAt, &doc.Accessed, &rank)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsJSON), &doc.Tags)
		doc.Embedding = blobToFloat32(embedBlob)
		score := float32(1.0 / (1.0 + math.Abs(rank)))
		results = append(results, domain.ScoredDocument{Document: &doc, Score: score})
	}
	if len(results) == 0 {
		return r.searchByTextLike(ctx, query, tags, notebook, limit)
	}
	return results, nil
}

func toFTSQuery(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		w = strings.Map(func(r rune) rune {
			if r == '"' || r == '(' || r == ')' || r == '^' {
				return -1
			}
			return r
		}, w)
		if len(w) > 0 {
			w = w + "*"
		}
		words[i] = w
	}
	return strings.Join(words, " ")
}

func (r *DocumentRepo) searchByTextLike(ctx context.Context, query string, tags []string, notebook string, limit int) ([]domain.ScoredDocument, error) {
	words := strings.Fields(query)
	qry := `SELECT id, title, content, tags, grupo, categoria, embedding, created_at, updated_at, accessed FROM documents WHERE 1=1`
	var conds []string
	var args []any

	for _, w := range words {
		conds = append(conds, "(title LIKE ? OR content LIKE ?)")
		args = append(args, `%`+w+`%`, `%`+w+`%`)
	}

	if len(tags) > 0 {
		for _, t := range tags {
			conds = append(conds, "tags LIKE ?")
			args = append(args, `%"`+t+`"%`)
		}
	}
	if notebook != "" {
		conds = append(conds, "grupo = ?")
		args = append(args, notebook)
	}

	if len(conds) > 0 {
		qry += " AND " + strings.Join(conds, " AND ")
	}

	qry += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, qry, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs, err := r.scanDocuments(rows)
	if err != nil {
		return nil, err
	}

	var results []domain.ScoredDocument
	for _, d := range docs {
		score := scoreKeywordMatch(d, words)
		results = append(results, domain.ScoredDocument{Document: d, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func scoreKeywordMatch(doc *domain.Document, words []string) float32 {
	title := strings.ToLower(doc.Title)
	content := strings.ToLower(doc.Content)
	var score float32
	for _, w := range words {
		w = strings.ToLower(w)
		if w == "" {
			continue
		}
		titleCount := float32(strings.Count(title, w))
		contentCount := float32(strings.Count(content, w))
		if titleCount > 0 {
			score += titleCount*5 + contentCount
		} else if contentCount > 0 {
			score += contentCount
		}
	}
	if score > 100 {
		score = 100
	}
	return score / 100
}

func (r *DocumentRepo) IncrementAccessed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE documents SET accessed = accessed + 1 WHERE id = ?`, id)
	return err
}

func (r *DocumentRepo) GetAllTags(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tags FROM documents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := map[string]int{}
	for rows.Next() {
		var tagsJSON string
		if err := rows.Scan(&tagsJSON); err != nil {
			return nil, err
		}
		var list []string
		json.Unmarshal([]byte(tagsJSON), &list)
		for _, t := range list {
			tags[t]++
		}
	}
	return tags, nil
}

func (r *DocumentRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM document_links`)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM documents`)
	if err != nil {
		return err
	}
	if r.useFTS {
		_, err = r.db.ExecContext(ctx, `DELETE FROM docs_fts`)
	}
	return err
}

func (r *DocumentRepo) ExportAll(ctx context.Context) ([]*domain.Document, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, content, tags, grupo, categoria, embedding, created_at, updated_at, accessed
		 FROM documents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanDocuments(rows)
}

func (r *DocumentRepo) ImportMany(ctx context.Context, docs []*domain.Document) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		tagsJSON, _ := json.Marshal(doc.Tags)
		embedBlob := float32ToBlob(doc.Embedding)
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO documents (id, title, content, tags, grupo, categoria, embedding, created_at, updated_at, accessed)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
doc.ID, doc.Title, doc.Content, string(tagsJSON), doc.Notebook, doc.Category, embedBlob,
			doc.CreatedAt, doc.UpdatedAt, doc.Accessed,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if r.useFTS {
		r.rebuildFTS(ctx)
	}
	return nil
}

func (r *DocumentRepo) DB() *sql.DB {
	return r.db
}

func (r *DocumentRepo) buildWhere(f domain.ListFilter) (string, []any) {
	var conds []string
	var args []any
	if len(f.Tags) > 0 {
		for _, t := range f.Tags {
			conds = append(conds, "tags LIKE ?")
			args = append(args, `%"`+t+`"%`)
		}
	}
	if f.Notebook != "" {
		conds = append(conds, "grupo = ?")
		args = append(args, f.Notebook)
	}
	if f.Category != "" {
		conds = append(conds, "categoria = ?")
		args = append(args, f.Category)
	}
	return strings.Join(conds, " AND "), args
}

func (r *DocumentRepo) sortClause(s string) string {
	switch s {
	case "accessed":
		return "accessed DESC"
	case "alpha":
		return "title ASC"
	default:
		return "created_at DESC"
	}
}

func (r *DocumentRepo) scanDocument(row *sql.Row) (*domain.Document, error) {
	var doc domain.Document
	var tagsJSON string
	var embedBlob []byte
	err := row.Scan(&doc.ID, &doc.Title, &doc.Content, &tagsJSON, &doc.Notebook, &doc.Category,
		&embedBlob, &doc.CreatedAt, &doc.UpdatedAt, &doc.Accessed)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tagsJSON), &doc.Tags)
	doc.Embedding = blobToFloat32(embedBlob)
	return &doc, nil
}

func (r *DocumentRepo) scanDocuments(rows *sql.Rows) ([]*domain.Document, error) {
	var docs []*domain.Document
	for rows.Next() {
		var doc domain.Document
		var tagsJSON string
		var embedBlob []byte
		err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &tagsJSON, &doc.Notebook, &doc.Category,
			&embedBlob, &doc.CreatedAt, &doc.UpdatedAt, &doc.Accessed)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsJSON), &doc.Tags)
		doc.Embedding = blobToFloat32(embedBlob)
		docs = append(docs, &doc)
	}
	return docs, nil
}

func float32ToBlob(f []float32) []byte {
	if len(f) == 0 {
		return nil
	}
	blob := make([]byte, len(f)*4)
	for i, v := range f {
		bits := math.Float32bits(v)
		blob[i*4] = byte(bits)
		blob[i*4+1] = byte(bits >> 8)
		blob[i*4+2] = byte(bits >> 16)
		blob[i*4+3] = byte(bits >> 24)
	}
	return blob
}

func blobToFloat32(blob []byte) []float32 {
	if len(blob) == 0 {
		return nil
	}
	f := make([]float32, len(blob)/4)
	for i := range f {
		bits := uint32(blob[i*4]) | uint32(blob[i*4+1])<<8 | uint32(blob[i*4+2])<<16 | uint32(blob[i*4+3])<<24
		f[i] = math.Float32frombits(bits)
	}
	return f
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(normA))*math.Sqrt(float64(normB)))
}
