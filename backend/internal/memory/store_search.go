package memory

import (
	"context"
	"math"
	"sort"
	"strings"

	"kubelens-backend/internal/model"
)

func (s *Store) SearchContext(ctx context.Context, query string) []model.MemoryRunbook {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	terms := searchTerms(needle)
	runbooks, embedder := s.snapshotRunbooks()
	if len(runbooks) == 0 {
		return nil
	}

	if needle != "" && embedder != nil && len(runbooks[0].Embedding) > 0 {
		queryEmbedding, err := embedder.Embed(ctx, needle)
		if err == nil {
			if ranked := rankRunbooksByEmbedding(runbooks, queryEmbedding); len(ranked) > 0 {
				return ranked
			}
		} else if s.logger != nil {
			s.logger.Warn("memory query embedding failed", "query", query, "error", err.Error())
		}
	}

	return rankRunbooksByKeyword(runbooks, needle, terms)
}

func (s *Store) snapshotRunbooks() ([]model.MemoryRunbook, EmbeddingClient) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runbooks := make([]model.MemoryRunbook, 0, len(s.runbooks))
	for _, runbook := range s.runbooks {
		runbooks = append(runbooks, cloneRunbook(runbook))
	}
	return runbooks, s.embeddingClient
}

func runbookMatches(runbook model.MemoryRunbook, needle string) bool {
	if strings.Contains(strings.ToLower(runbook.Title), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(runbook.Description), needle) {
		return true
	}
	for _, tag := range runbook.Tags {
		if strings.Contains(strings.ToLower(tag), needle) {
			return true
		}
	}
	for _, step := range runbook.Steps {
		if strings.Contains(strings.ToLower(step), needle) {
			return true
		}
	}
	return false
}

func searchTerms(query string) []string {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		term := strings.TrimSpace(strings.ToLower(field))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	return out
}

func runbookMatchScore(runbook model.MemoryRunbook, needle string, terms []string) int {
	if strings.TrimSpace(needle) == "" {
		return 0
	}
	if !runbookMatches(runbook, needle) {
		return 0
	}

	score := 0
	title := strings.ToLower(runbook.Title)
	description := strings.ToLower(runbook.Description)
	tags := make([]string, 0, len(runbook.Tags))
	for _, tag := range runbook.Tags {
		tags = append(tags, strings.ToLower(tag))
	}
	steps := make([]string, 0, len(runbook.Steps))
	for _, step := range runbook.Steps {
		steps = append(steps, strings.ToLower(step))
	}

	if strings.Contains(title, needle) {
		score += 12
	}
	if strings.Contains(description, needle) {
		score += 4
	}

	for _, term := range terms {
		if strings.Contains(title, term) {
			score += 8
		}
		if strings.Contains(description, term) {
			score += 3
		}
		for _, tag := range tags {
			if tag == term {
				score += 7
				continue
			}
			if strings.Contains(tag, term) {
				score += 5
			}
		}
		for _, step := range steps {
			if strings.Contains(step, term) {
				score += 2
				break
			}
		}
	}

	return score
}

func rankRunbooksByKeyword(runbooks []model.MemoryRunbook, needle string, terms []string) []model.MemoryRunbook {
	type scoredRunbook struct {
		runbook model.MemoryRunbook
		score   int
	}

	candidates := make([]scoredRunbook, 0, len(runbooks))
	for _, runbook := range runbooks {
		score := 0
		if needle != "" {
			score = runbookMatchScore(runbook, needle, terms)
			if score == 0 {
				continue
			}
		}
		candidates = append(candidates, scoredRunbook{
			runbook: runbook,
			score:   score,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].runbook.UsageCount != candidates[j].runbook.UsageCount {
			return candidates[i].runbook.UsageCount > candidates[j].runbook.UsageCount
		}
		return candidates[i].runbook.UpdatedAt > candidates[j].runbook.UpdatedAt
	})

	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	out := make([]model.MemoryRunbook, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.runbook)
	}
	return out
}

func rankRunbooksByEmbedding(runbooks []model.MemoryRunbook, queryEmbedding []float32) []model.MemoryRunbook {
	type scoredRunbook struct {
		runbook    model.MemoryRunbook
		similarity float32
	}

	candidates := make([]scoredRunbook, 0, len(runbooks))
	hasSimilarity := false
	for _, runbook := range runbooks {
		similarity := cosineSimilarity(queryEmbedding, runbook.Embedding)
		if similarity > 0 {
			hasSimilarity = true
		}
		candidates = append(candidates, scoredRunbook{
			runbook:    runbook,
			similarity: similarity,
		})
	}
	if !hasSimilarity {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].similarity != candidates[j].similarity {
			return candidates[i].similarity > candidates[j].similarity
		}
		if candidates[i].runbook.UsageCount != candidates[j].runbook.UsageCount {
			return candidates[i].runbook.UsageCount > candidates[j].runbook.UsageCount
		}
		return candidates[i].runbook.UpdatedAt > candidates[j].runbook.UpdatedAt
	})

	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	out := make([]model.MemoryRunbook, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.runbook)
	}
	return out
}

func cosineSimilarity(a, b []float32) float32 {
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
