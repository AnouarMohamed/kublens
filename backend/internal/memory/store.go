package memory

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kubelens-backend/internal/model"
)

type EmbeddingClient interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Store struct {
	filePath        string
	logger          *slog.Logger
	now             func() time.Time
	embeddingClient EmbeddingClient

	mu       sync.RWMutex
	counter  uint64
	runbooks []model.MemoryRunbook
	fixes    []model.MemoryFixPattern
}

func New(filePath string, logger *slog.Logger) *Store {
	return NewWithEmbeddings(filePath, logger, nil)
}

func NewWithEmbeddings(filePath string, logger *slog.Logger, embedder EmbeddingClient) *Store {
	path := strings.TrimSpace(filePath)
	if path == "" {
		path = filepath.Clean("data/memory-runbooks.json")
	}
	if logger == nil {
		logger = slog.Default()
	}

	store := &Store{
		filePath:        path,
		logger:          logger,
		now:             time.Now,
		embeddingClient: embedder,
		runbooks:        make([]model.MemoryRunbook, 0, 128),
		fixes:           make([]model.MemoryFixPattern, 0, 256),
	}
	store.load()
	return store
}

func (s *Store) Search(query string) []model.MemoryRunbook {
	return s.SearchContext(context.Background(), query)
}

func (s *Store) AddRunbook(ctx context.Context, runbook model.MemoryRunbook) (model.MemoryRunbook, error) {
	if s == nil {
		return model.MemoryRunbook{}, os.ErrInvalid
	}

	normalized, err := normalizeRunbookRequest(model.MemoryRunbookUpsertRequest{
		Title:       runbook.Title,
		Tags:        runbook.Tags,
		Description: runbook.Description,
		Steps:       runbook.Steps,
	})
	if err != nil {
		return model.MemoryRunbook{}, err
	}

	embedding, err := s.embedRunbook(ctx, normalized)
	if err != nil && s.logger != nil {
		s.logger.Warn("memory runbook embedding failed", "title", normalized.Title, "error", err.Error())
	}
	if len(embedding) > 0 {
		normalized.Embedding = embedding
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	nowAt := s.now().UTC().Format(time.RFC3339)
	normalized.ID = "rbk-" + formatCounter(s.counter)
	normalized.CreatedAt = nowAt
	normalized.UpdatedAt = nowAt
	normalized.UsageCount = 0
	s.runbooks = append(s.runbooks, normalized)
	s.persistLocked()
	return cloneRunbook(normalized), nil
}

func (s *Store) embedRunbook(ctx context.Context, runbook model.MemoryRunbook) ([]float32, error) {
	if s == nil || s.embeddingClient == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	vector, err := s.embeddingClient.Embed(ctx, runbookEmbeddingText(runbook))
	if err != nil {
		return nil, err
	}
	return append([]float32(nil), vector...), nil
}

func (s *Store) IncrementUsage(id string) bool {
	if s == nil {
		return false
	}

	needle := strings.TrimSpace(id)
	if needle == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.runbooks {
		if s.runbooks[i].ID != needle {
			continue
		}
		s.runbooks[i].UsageCount++
		s.runbooks[i].UpdatedAt = s.now().UTC().Format(time.RFC3339)
		s.persistLocked()
		return true
	}
	return false
}

func (s *Store) CreateRunbook(req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error) {
	if s == nil {
		return model.MemoryRunbook{}, os.ErrInvalid
	}

	runbook, err := normalizeRunbookRequest(req)
	if err != nil {
		return model.MemoryRunbook{}, err
	}
	return s.AddRunbook(context.Background(), runbook)
}

func (s *Store) UpdateRunbook(id string, req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error) {
	return s.updateRunbookContext(context.Background(), id, req)
}

func (s *Store) updateRunbookContext(ctx context.Context, id string, req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error) {
	if s == nil {
		return model.MemoryRunbook{}, os.ErrInvalid
	}

	needle := strings.TrimSpace(id)
	if needle == "" {
		return model.MemoryRunbook{}, os.ErrInvalid
	}
	normalized, err := normalizeRunbookRequest(req)
	if err != nil {
		return model.MemoryRunbook{}, err
	}

	embedding, embeddingErr := s.embedRunbook(ctx, normalized)
	if embeddingErr != nil && s.logger != nil {
		s.logger.Warn("memory runbook embedding failed", "id", needle, "title", normalized.Title, "error", embeddingErr.Error())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.runbooks {
		if s.runbooks[i].ID != needle {
			continue
		}
		s.runbooks[i].Title = normalized.Title
		s.runbooks[i].Tags = append([]string(nil), normalized.Tags...)
		s.runbooks[i].Description = normalized.Description
		s.runbooks[i].Steps = append([]string(nil), normalized.Steps...)
		if len(embedding) > 0 {
			s.runbooks[i].Embedding = append([]float32(nil), embedding...)
		}
		s.runbooks[i].UpdatedAt = s.now().UTC().Format(time.RFC3339)
		s.persistLocked()
		return cloneRunbook(s.runbooks[i]), nil
	}

	return model.MemoryRunbook{}, os.ErrNotExist
}

func (s *Store) ListFixes() []model.MemoryFixPattern {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]model.MemoryFixPattern, 0, len(s.fixes))
	for i := range s.fixes {
		out = append(out, cloneFix(s.fixes[i]))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].RecordedAt > out[j].RecordedAt
	})
	return out
}

func (s *Store) RecordFix(req model.MemoryFixCreateRequest, recordedBy string) (model.MemoryFixPattern, error) {
	if s == nil {
		return model.MemoryFixPattern{}, os.ErrInvalid
	}
	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	resource := strings.TrimSpace(req.Resource)
	if title == "" || description == "" || resource == "" {
		return model.MemoryFixPattern{}, os.ErrInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	nowAt := s.now().UTC().Format(time.RFC3339)
	fix := model.MemoryFixPattern{
		ID:          "fix-" + formatCounter(s.counter),
		IncidentID:  strings.TrimSpace(req.IncidentID),
		ProposalID:  strings.TrimSpace(req.ProposalID),
		Title:       title,
		Description: description,
		Resource:    resource,
		Kind:        req.Kind,
		RecordedBy:  defaultString(strings.TrimSpace(recordedBy), "unknown"),
		RecordedAt:  nowAt,
	}
	s.fixes = append(s.fixes, fix)
	s.persistLocked()
	return cloneFix(fix), nil
}
