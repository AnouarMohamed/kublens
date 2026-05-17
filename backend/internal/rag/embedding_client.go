package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

type EmbeddingClient struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewEmbeddingClient(baseURL, model string, httpClient *http.Client) (*EmbeddingClient, error) {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		trimmedBaseURL = defaultOllamaBaseURL
	}
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		trimmedModel = defaultEmbeddingModel
	}
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &EmbeddingClient{
		baseURL: trimmedBaseURL,
		model:   trimmedModel,
		client:  client,
	}, nil
}

func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if c == nil {
		return nil, errors.New("embedding client is not configured")
	}

	payload, err := json.Marshal(map[string]any{
		"model":  c.model,
		"prompt": text,
	})
	if err != nil {
		return nil, fmt.Errorf("encode embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		return nil, fmt.Errorf("embedding status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var out struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode embeddings: %w", err)
	}
	if len(out.Embedding) == 0 {
		return nil, errors.New("empty embeddings response")
	}

	vector := make([]float32, len(out.Embedding))
	for i, value := range out.Embedding {
		vector[i] = float32(value)
	}
	return vector, nil
}

func cosineSimilarity(a []float32, b []float32) float32 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float32
	var sumA float32
	var sumB float32
	for i := range a {
		dot += a[i] * b[i]
		sumA += a[i] * a[i]
		sumB += b[i] * b[i]
	}
	denom := float32(math.Sqrt(float64(sumA))) * float32(math.Sqrt(float64(sumB)))
	if denom == 0 {
		return 0
	}
	return dot / denom
}
