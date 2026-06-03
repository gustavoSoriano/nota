package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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
			id        TEXT PRIMARY KEY,
			title     TEXT NOT NULL,
			content   TEXT NOT NULL,
			tags      TEXT NOT NULL DEFAULT '[]',
			grupo     TEXT NOT NULL DEFAULT '',
			categoria TEXT NOT NULL DEFAULT '',
			embedding BLOB,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			accessed  INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS document_links (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			PRIMARY KEY (source_id, target_id),
			FOREIGN KEY (source_id) REFERENCES documents(id) ON DELETE CASCADE,
			FOREIGN KEY (target_id) REFERENCES documents(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_docs_grupo     ON documents(grupo);
		CREATE INDEX IF NOT EXISTS idx_docs_categoria ON documents(categoria);
		CREATE INDEX IF NOT EXISTS idx_docs_created   ON documents(created_at DESC);
	`)
	if err != nil {
		return err
	}

	// FTS5 com tags indexadas para busca por tag via texto
	_, ftsErr := r.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
			doc_id UNINDEXED,
			title,
			content,
			tags,
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

func (r *DocumentRepo) syncFTS(ctx context.Context, docID, title, content, tags string) {
	if !r.useFTS {
		return
	}
	r.db.ExecContext(ctx, `DELETE FROM docs_fts WHERE doc_id = ?`, docID)
	r.db.ExecContext(ctx, `INSERT INTO docs_fts (doc_id, title, content, tags) VALUES (?, ?, ?, ?)`,
		docID, title, content, tags)
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
	r.db.ExecContext(ctx, `
		INSERT INTO docs_fts (doc_id, title, content, tags)
		SELECT id, title, content, tags FROM documents
	`)
}

func (r *DocumentRepo) Create(ctx context.Context, doc *domain.Document) error {
	tagsJSON, _ := json.Marshal(doc.Tags)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO documents (id, title, content, tags, grupo, categoria, created_at, updated_at, accessed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Title, doc.Content, string(tagsJSON), doc.Notebook, doc.Category,
		doc.CreatedAt, doc.UpdatedAt, doc.Accessed,
	)
	if err == nil {
		r.syncFTS(ctx, doc.ID, doc.Title, doc.Content, tagsToText(doc.Tags))
	}
	return err
}

func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*domain.Document, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, title, content, tags, grupo, categoria, created_at, updated_at, accessed
		 FROM documents WHERE id = ?`, id)
	return r.scanDocument(row)
}

func (r *DocumentRepo) Update(ctx context.Context, doc *domain.Document) error {
	tagsJSON, _ := json.Marshal(doc.Tags)
	_, err := r.db.ExecContext(ctx,
		`UPDATE documents SET title=?, content=?, tags=?, grupo=?, categoria=?, updated_at=?, accessed=?
		 WHERE id=?`,
		doc.Title, doc.Content, string(tagsJSON), doc.Notebook, doc.Category,
		doc.UpdatedAt, doc.Accessed, doc.ID,
	)
	if err == nil {
		r.syncFTS(ctx, doc.ID, doc.Title, doc.Content, tagsToText(doc.Tags))
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
	qry := `SELECT id, title, content, tags, grupo, categoria, created_at, updated_at, accessed
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

func (r *DocumentRepo) SearchByText(ctx context.Context, query string, tags []string, notebook string, category string, limit int) ([]domain.ScoredDocument, error) {
	if limit <= 0 {
		limit = 40
	}
	if r.useFTS {
		return r.searchByTextFTS(ctx, query, tags, notebook, category, limit)
	}
	return r.searchByTextLike(ctx, query, tags, notebook, category, limit)
}

func (r *DocumentRepo) searchByTextFTS(ctx context.Context, query string, tags []string, notebook string, category string, limit int) ([]domain.ScoredDocument, error) {
	ftsQuery := toFTSQuery(query)
	if ftsQuery == "" {
		return r.searchByTextLike(ctx, query, tags, notebook, category, limit)
	}

	qry := `
		SELECT d.id, d.title, d.content, d.tags, d.grupo, d.categoria, d.created_at, d.updated_at, d.accessed,
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
	if category != "" {
		conds = append(conds, "d.categoria = ?")
		args = append(args, category)
	}
	if len(conds) > 0 {
		qry += " AND " + strings.Join(conds, " AND ")
	}

	qry += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, qry, args...)
	if err != nil {
		return r.searchByTextLike(ctx, query, tags, notebook, category, limit)
	}
	defer rows.Close()

	type rawResult struct {
		doc  domain.Document
		rank float64
	}
	var raw []rawResult

	for rows.Next() {
		var doc domain.Document
		var tagsJSON string
		var rank float64
		err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &tagsJSON, &doc.Notebook, &doc.Category,
			&doc.CreatedAt, &doc.UpdatedAt, &doc.Accessed, &rank)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsJSON), &doc.Tags)
		raw = append(raw, rawResult{doc: doc, rank: rank})
	}

	if len(raw) == 0 {
		return r.searchByTextLike(ctx, query, tags, notebook, category, limit)
	}

	// Normalizar BM25: rank é negativo (mais negativo = melhor match)
	// min-max → inverter → [0,1] onde 1 é o melhor
	minRank := raw[0].rank
	maxRank := raw[0].rank
	for _, rr := range raw {
		if rr.rank < minRank {
			minRank = rr.rank
		}
		if rr.rank > maxRank {
			maxRank = rr.rank
		}
	}

	var results []domain.ScoredDocument
	for _, rr := range raw {
		var score float32
		if maxRank == minRank {
			score = 1.0
		} else {
			// rank mais negativo → score mais alto
			score = float32((rr.rank - maxRank) / (minRank - maxRank))
		}
		doc := rr.doc
		results = append(results, domain.ScoredDocument{Document: &doc, Score: score})
	}

	return results, nil
}

func toFTSQuery(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Preservar frases entre aspas
	var parts []string
	remaining := s
	for {
		start := strings.Index(remaining, `"`)
		if start == -1 {
			break
		}
		// Adicionar palavras antes das aspas
		before := strings.TrimSpace(remaining[:start])
		if before != "" {
			for _, w := range strings.Fields(before) {
				w = sanitizeFTSWord(w)
				if w != "" {
					parts = append(parts, w+"*")
				}
			}
		}
		remaining = remaining[start+1:]
		end := strings.Index(remaining, `"`)
		if end == -1 {
			// Aspas não fechadas: tratar resto como palavras normais
			remaining = remaining
			break
		}
		phrase := strings.TrimSpace(remaining[:end])
		if phrase != "" {
			parts = append(parts, `"`+phrase+`"`)
		}
		remaining = remaining[end+1:]
	}

	// Processar palavras restantes
	for _, w := range strings.Fields(remaining) {
		w = sanitizeFTSWord(w)
		if w != "" {
			parts = append(parts, w+"*")
		}
	}

	return strings.Join(parts, " ")
}

func sanitizeFTSWord(w string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '(' || r == ')' || r == '^' || r == '*' {
			return -1
		}
		return r
	}, w)
}

func (r *DocumentRepo) searchByTextLike(ctx context.Context, query string, tags []string, notebook string, category string, limit int) ([]domain.ScoredDocument, error) {
	words := strings.Fields(query)
	qry := `SELECT id, title, content, tags, grupo, categoria, created_at, updated_at, accessed FROM documents WHERE 1=1`
	var conds []string
	var args []any

	for _, w := range words {
		conds = append(conds, "(title LIKE ? OR content LIKE ? OR tags LIKE ?)")
		args = append(args, `%`+w+`%`, `%`+w+`%`, `%`+w+`%`)
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
	if category != "" {
		conds = append(conds, "categoria = ?")
		args = append(args, category)
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
	tags := strings.ToLower(strings.Join(doc.Tags, " "))
	var score float32
	for _, w := range words {
		w = strings.ToLower(w)
		if w == "" {
			continue
		}
		titleCount := float32(strings.Count(title, w))
		contentCount := float32(strings.Count(content, w))
		tagCount := float32(strings.Count(tags, w))
		score += titleCount*5 + tagCount*3 + contentCount
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
		`SELECT id, title, content, tags, grupo, categoria, created_at, updated_at, accessed
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
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO documents (id, title, content, tags, grupo, categoria, created_at, updated_at, accessed)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.ID, doc.Title, doc.Content, string(tagsJSON), doc.Notebook, doc.Category,
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
	err := row.Scan(&doc.ID, &doc.Title, &doc.Content, &tagsJSON, &doc.Notebook, &doc.Category,
		&doc.CreatedAt, &doc.UpdatedAt, &doc.Accessed)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tagsJSON), &doc.Tags)
	return &doc, nil
}

func (r *DocumentRepo) scanDocuments(rows *sql.Rows) ([]*domain.Document, error) {
	var docs []*domain.Document
	for rows.Next() {
		var doc domain.Document
		var tagsJSON string
		err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &tagsJSON, &doc.Notebook, &doc.Category,
			&doc.CreatedAt, &doc.UpdatedAt, &doc.Accessed)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsJSON), &doc.Tags)
		docs = append(docs, &doc)
	}
	return docs, nil
}

// tagsToText converte slice de tags em texto para indexação FTS
func tagsToText(tags []string) string {
	return strings.Join(tags, " ")
}
