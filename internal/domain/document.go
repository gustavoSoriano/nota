package domain

import (
	"strings"
	"time"
)

type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	Notebook  string    `json:"notebook,omitempty"`
	Category  string    `json:"category,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Accessed  int       `json:"accessed"`
}

func (d *Document) ExtractTitle() string {
	for _, line := range splitLines(d.Content) {
		trimmed := trimHashPrefix(line)
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			return truncated(trimmed, 120)
		}
	}
	if d.Title != "" {
		return d.Title
	}
	return "untitled"
}
