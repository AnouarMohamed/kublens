package rag

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"kubelens-backend/internal/redact"
)

func (s *Service) buildIndex(ctx context.Context) ([]chunk, [][]float32) {
	results := make([][]chunk, len(s.sources))
	var wg sync.WaitGroup
	wg.Add(len(s.sources))
	for i := range s.sources {
		i := i
		source := s.sources[i]
		go func() {
			defer wg.Done()

			text, err := s.fetchSourceText(ctx, source)
			if err != nil {
				if s.logger != nil {
					s.logger.Warn("rag source fetch failed", "source", source.Source, "url", source.URL, "error", redact.Error(err))
				}
				text = source.Fallback
			}

			chunks := make([]chunk, 0, 3)
			for _, part := range chunkText(text, defaultChunkSize, defaultChunkOverlap) {
				item := newChunk(source, part)
				if item.text == "" {
					continue
				}
				chunks = append(chunks, item)
			}
			results[i] = chunks
		}()
	}
	wg.Wait()

	out := make([]chunk, 0, len(s.sources)*3)
	for _, chunks := range results {
		out = append(out, chunks...)
	}

	return out, s.buildEmbeddings(ctx, out)
}

func (s *Service) fallbackIndex() ([]chunk, [][]float32) {
	out := make([]chunk, 0, len(s.sources))
	for _, source := range s.sources {
		for _, part := range chunkText(source.Fallback, defaultChunkSize, defaultChunkOverlap) {
			item := newChunk(source, part)
			if item.text == "" {
				continue
			}
			out = append(out, item)
		}
	}
	return out, s.buildEmbeddings(context.Background(), out)
}

func newChunk(source SourceDoc, raw string) chunk {
	text := strings.TrimSpace(spacePattern.ReplaceAllString(raw, " "))
	if text == "" {
		return chunk{}
	}
	lower := strings.ToLower(text)
	terms := tokenize(lower)
	tokenSet := make(map[string]struct{}, len(terms))
	for _, token := range terms {
		tokenSet[token] = struct{}{}
	}
	return chunk{
		source:    source.Source,
		title:     source.Title,
		url:       source.URL,
		text:      text,
		textLower: lower,
		tokenSet:  tokenSet,
	}
}

func (s *Service) buildEmbeddings(ctx context.Context, chunks []chunk) [][]float32 {
	if s.embeddingClient == nil || len(chunks) == 0 {
		return nil
	}

	embeddings := make([][]float32, len(chunks))
	for i := range chunks {
		vector, err := s.embeddingClient.Embed(ctx, chunks[i].text)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("rag embeddings failed", "error", redact.Error(err))
			}
			return nil
		}
		embeddings[i] = vector
	}
	return embeddings
}

func (s *Service) semanticScores(
	ctx context.Context,
	query string,
	candidates []int,
	embeddings [][]float32,
) (map[int]float64, bool) {
	if s.embeddingClient == nil || len(candidates) == 0 || len(embeddings) == 0 {
		return nil, false
	}

	queryEmbedding, err := s.embeddingClient.Embed(ctx, query)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("rag query embedding failed", "error", redact.Error(err))
		}
		return nil, false
	}
	if len(queryEmbedding) == 0 {
		return nil, false
	}

	scores := make(map[int]float64, len(candidates))
	for _, index := range candidates {
		if index < 0 || index >= len(embeddings) {
			continue
		}
		vector := embeddings[index]
		if len(vector) == 0 || len(vector) != len(queryEmbedding) {
			continue
		}
		score := cosineSimilarity(queryEmbedding, vector)
		scores[index] = float64(score)
	}
	if len(scores) == 0 {
		return nil, false
	}
	return scores, true
}

func (s *Service) fetchSourceText(ctx context.Context, source SourceDoc) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "KubeLens-RAG/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBodyBytes))
	if err != nil {
		return "", err
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	text := string(body)
	if strings.Contains(contentType, "html") || strings.Contains(text, "<html") {
		return htmlToText(text), nil
	}
	return normalizeText(text), nil
}
