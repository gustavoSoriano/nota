package domain

import "time"

type Document struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Tags       []string  `json:"tags"`
	Grupo      string    `json:"grupo,omitempty"`
	Categoria  string    `json:"categoria,omitempty"`
	Embedding  []float32 `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Accessed   int       `json:"accessed"`
}

func (d *Document) ExtractTitle() string {
	if d.Title != "" {
		return d.Title
	}
	for _, line := range splitLines(d.Content) {
		trimmed := trimHashPrefix(line)
		if trimmed != "" {
			return truncated(trimmed, 120)
		}
	}
	return "untitled"
}
