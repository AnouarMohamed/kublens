package rag

import (
	"context"
	"sort"
	"strings"
	"time"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/redact"
)

func (s *Service) Retrieve(ctx context.Context, query string, limit int) []model.DocumentationReference {
	if !s.Enabled() {
		return nil
	}

	started := s.now()
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	if limit <= 0 {
		limit = defaultResultLimit
	}
	if limit > maxResultLimit {
		limit = maxResultLimit
	}

	s.ensureLoaded(ctx)

	chunks, embeddings, tokenIdx, expiresAt, indexedAt := s.snapshotIndex()
	if len(chunks) == 0 {
		s.recordRetrieval(query, nil, false, nil, 0, 0, started)
		return nil
	}
	s.warnIfStaleIndex(expiresAt, indexedAt)

	parsed := buildRetrievalQuery(query)
	if len(parsed.expandedTerms) == 0 {
		s.recordRetrieval(query, parsed.terms, false, nil, 0, 0, started)
		return nil
	}

	candidates := candidateIndexes(parsed.expandedTerms, tokenIdx, len(chunks))
	if len(candidates) == 0 {
		candidates = make([]int, len(chunks))
		for i := range chunks {
			candidates[i] = i
		}
	}

	sourceHints := buildSourceRoutingHints(parsed.expandedTerms, parsed.rawLower)

	lexicalScores := make(map[int]float64, len(candidates))
	maxLexical := 0.0
	for _, index := range candidates {
		score := matchScore(chunks[index], parsed.rawLower, parsed.terms, parsed.expandedTerms)
		if score <= 0 {
			continue
		}
		lexicalScores[index] = score
		if score > maxLexical {
			maxLexical = score
		}
	}

	semanticScores, hasSemantic := s.semanticScores(ctx, query, candidates, embeddings)

	combinedIndexes := make(map[int]struct{}, len(lexicalScores)+len(semanticScores))
	for index := range lexicalScores {
		combinedIndexes[index] = struct{}{}
	}
	for index := range semanticScores {
		combinedIndexes[index] = struct{}{}
	}
	if len(combinedIndexes) == 0 {
		s.recordRetrieval(query, parsed.terms, hasSemantic, nil, 0, len(candidates), started)
		return nil
	}

	type scored struct {
		index         int
		total         float64
		lexicalNorm   float64
		semanticNorm  float64
		queryCoverage float64
		sourceBoost   float64
		feedbackBoost float64
	}

	ranked := make([]scored, 0, len(combinedIndexes))
	for index := range combinedIndexes {
		chunk := chunks[index]
		lexical := lexicalScores[index]
		lexicalNorm := 0.0
		if maxLexical > 0 {
			lexicalNorm = lexical / maxLexical
		}

		semanticNorm := 0.0
		if hasSemantic {
			semanticNorm = normalizeSemanticScore(semanticScores[index])
		}

		coverage := queryCoverage(chunk, parsed.terms)
		sourceBoost := sourceRouteBoost(chunk, sourceHints)
		feedbackBoost := s.feedbackBoostForQuery(chunk.url, parsed.expandedTerms)
		total := lexicalNorm*0.58 + semanticNorm*0.22 + coverage*0.05 + sourceBoost*0.10 + feedbackBoost*0.05

		if len(parsed.terms) > 0 && lexical <= 0 {
			if !hasSemantic || semanticNorm < 0.62 {
				continue
			}
			total *= 0.88
		}
		if total <= 0 {
			continue
		}
		ranked = append(ranked, scored{
			index:         index,
			total:         total,
			lexicalNorm:   lexicalNorm,
			semanticNorm:  semanticNorm,
			queryCoverage: coverage,
			sourceBoost:   sourceBoost,
			feedbackBoost: feedbackBoost,
		})
	}
	if len(ranked) == 0 {
		s.recordRetrieval(query, parsed.terms, hasSemantic, nil, 0, len(candidates), started)
		return nil
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].total == ranked[j].total {
			if ranked[i].lexicalNorm == ranked[j].lexicalNorm {
				if ranked[i].semanticNorm == ranked[j].semanticNorm {
					left := chunks[ranked[i].index]
					right := chunks[ranked[j].index]
					return left.title < right.title
				}
				return ranked[i].semanticNorm > ranked[j].semanticNorm
			}
			return ranked[i].lexicalNorm > ranked[j].lexicalNorm
		}
		return ranked[i].total > ranked[j].total
	})

	refs := make([]model.DocumentationReference, 0, limit)
	traceResults := make([]retrievalTraceResult, 0, minInt(len(ranked), 5))
	seenURL := map[string]struct{}{}
	for _, item := range ranked {
		if len(refs) >= limit {
			break
		}
		chunk := chunks[item.index]
		if _, exists := seenURL[chunk.url]; exists {
			continue
		}
		seenURL[chunk.url] = struct{}{}
		snippetTerms := parsed.terms
		if len(snippetTerms) == 0 {
			snippetTerms = parsed.expandedTerms
		}
		refs = append(refs, model.DocumentationReference{
			Title:   chunk.title,
			URL:     chunk.url,
			Source:  chunk.source,
			Snippet: bestSnippet(chunk.text, snippetTerms, 260),
		})
		if len(traceResults) < cap(traceResults) {
			traceResults = append(traceResults, retrievalTraceResult{
				title:         chunk.title,
				url:           chunk.url,
				source:        chunk.source,
				final:         item.total,
				lexical:       item.lexicalNorm,
				semantic:      item.semanticNorm,
				coverage:      item.queryCoverage,
				sourceBoost:   item.sourceBoost,
				feedbackBoost: item.feedbackBoost,
			})
		}
	}

	s.recordRetrieval(query, parsed.terms, hasSemantic, traceResults, len(refs), len(candidates), started)
	return refs
}

func (s *Service) ensureLoaded(ctx context.Context) {
	if s.hasFreshIndex() {
		return
	}

	_, err, _ := s.group.Do("refresh", func() (any, error) {
		if s.hasFreshIndex() {
			return nil, nil
		}

		chunks, embeddings := s.buildIndex(ctx)
		if len(chunks) == 0 {
			chunks, embeddings = s.fallbackIndex()
		}
		if len(chunks) == 0 {
			return nil, nil
		}

		s.mu.Lock()
		s.chunks = chunks
		s.embeddings = embeddings
		s.tokenIdx = buildTokenIndex(chunks)
		s.expiresAt = s.now().Add(s.refreshInterval)
		s.indexedAt = s.now()
		s.staleWarn = time.Time{}
		s.mu.Unlock()
		return nil, nil
	})
	if err != nil && s.logger != nil {
		s.logger.Warn("rag refresh failed", "error", redact.Error(err))
	}
}

func (s *Service) hasFreshIndex() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks) > 0 && s.now().Before(s.expiresAt)
}

func (s *Service) snapshotIndex() ([]chunk, [][]float32, map[string][]int, time.Time, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chunks, s.embeddings, s.tokenIdx, s.expiresAt, s.indexedAt
}

func (s *Service) warnIfStaleIndex(expiresAt, indexedAt time.Time) {
	if expiresAt.IsZero() || s.logger == nil {
		return
	}
	now := s.now()
	if now.Before(expiresAt) {
		return
	}

	s.mu.Lock()
	if !s.staleWarn.IsZero() && now.Sub(s.staleWarn) < 5*time.Minute {
		s.mu.Unlock()
		return
	}
	s.staleWarn = now
	s.mu.Unlock()

	age := "unknown"
	if !indexedAt.IsZero() {
		age = now.Sub(indexedAt).Round(time.Second).String()
	}
	s.logger.Warn("rag index is stale; serving potentially outdated references",
		"indexed_at", indexedAt,
		"expires_at", expiresAt,
		"index_age", age,
	)
}
