package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/soriano/nota/internal/domain"
)

type client struct {
	baseURL string
	model   string
	http    *http.Client
}

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type pullRequest struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

type tagResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func New(baseURL, model string) domain.EmbeddingService {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *client) Generate(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(embedRequest{Model: c.model, Prompt: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(b))
	}
	var emb embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&emb); err != nil {
		return nil, fmt.Errorf("decoding embedding: %w", err)
	}
	return emb.Embedding, nil
}

func (c *client) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		emb, err := c.Generate(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("embedding batch item %d: %w", i, err)
		}
		results[i] = emb
	}
	return results, nil
}

func (c *client) CheckAvailability(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not available: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var tags tagResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return err
	}
	found := false
	for _, m := range tags.Models {
		if strings.HasPrefix(m.Name, c.model) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("model %s not found in ollama, run: ollama pull %s", c.model, c.model)
	}
	return nil
}

func PullModel(ctx context.Context, baseURL, model string) error {
	c := &http.Client{Timeout: 10 * time.Minute}
	body, _ := json.Marshal(pullRequest{Name: model, Stream: false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("pulling model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
