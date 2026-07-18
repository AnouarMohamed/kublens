package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

type SQLStore struct {
	db              *sql.DB
	dialect         storesql.Dialect
	logger          *slog.Logger
	now             func() time.Time
	embeddingClient EmbeddingClient
	seq             atomic.Uint64
}

func NewSQLStore(db *sql.DB, dialect storesql.Dialect, logger *slog.Logger, embedder EmbeddingClient) *SQLStore {
	if db == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if dialect == "" {
		dialect = storesql.DialectSQLite
	}
	return &SQLStore{
		db:              db,
		dialect:         dialect,
		logger:          logger,
		now:             time.Now,
		embeddingClient: embedder,
	}
}

func (s *SQLStore) Search(query string) []model.MemoryRunbook {
	return s.SearchContext(context.Background(), query)
}

func (s *SQLStore) SearchContext(ctx context.Context, query string) []model.MemoryRunbook {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runbooks, err := s.listRunbooks(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("sql memory runbook search failed", "error", err.Error())
		}
		return nil
	}
	if len(runbooks) == 0 {
		return nil
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	terms := searchTerms(needle)
	if needle != "" && s.embeddingClient != nil && len(runbooks[0].Embedding) > 0 {
		queryEmbedding, err := s.embeddingClient.Embed(ctx, needle)
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

func (s *SQLStore) AddRunbook(ctx context.Context, runbook model.MemoryRunbook) (model.MemoryRunbook, error) {
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

	nowAt := s.now().UTC().Format(time.RFC3339)
	normalized.ID = s.nextID("rbk")
	normalized.CreatedAt = nowAt
	normalized.UpdatedAt = nowAt

	if err := s.insertRunbook(ctx, normalized); err != nil {
		return model.MemoryRunbook{}, err
	}
	return cloneRunbook(normalized), nil
}

func (s *SQLStore) CreateRunbook(req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error) {
	if s == nil {
		return model.MemoryRunbook{}, os.ErrInvalid
	}
	runbook, err := normalizeRunbookRequest(req)
	if err != nil {
		return model.MemoryRunbook{}, err
	}
	return s.AddRunbook(context.Background(), runbook)
}

func (s *SQLStore) UpdateRunbook(id string, req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error) {
	return s.updateRunbookContext(context.Background(), id, req)
}

func (s *SQLStore) updateRunbookContext(ctx context.Context, id string, req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error) {
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
	if len(embedding) > 0 {
		normalized.Embedding = embedding
	}

	existing, ok, err := s.getRunbook(ctx, needle)
	if err != nil {
		return model.MemoryRunbook{}, err
	}
	if !ok {
		return model.MemoryRunbook{}, os.ErrNotExist
	}
	if len(normalized.Embedding) == 0 {
		normalized.Embedding = existing.Embedding
	}

	normalized.ID = needle
	normalized.UsageCount = existing.UsageCount
	normalized.CreatedAt = existing.CreatedAt
	normalized.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.updateRunbook(ctx, normalized); err != nil {
		return model.MemoryRunbook{}, err
	}
	return cloneRunbook(normalized), nil
}

func (s *SQLStore) IncrementUsage(id string) bool {
	if s == nil {
		return false
	}
	needle := strings.TrimSpace(id)
	if needle == "" {
		return false
	}
	result, err := s.db.ExecContext(
		context.Background(),
		s.bind(`UPDATE memory_runbooks
		    SET usage_count = usage_count + 1, updated_at = ?
		  WHERE id = ?`),
		s.now().UTC().Format(time.RFC3339),
		needle,
	)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("sql memory usage update failed", "id", needle, "error", err.Error())
		}
		return false
	}
	affected, err := result.RowsAffected()
	return err == nil && affected > 0
}

func (s *SQLStore) ListFixes() []model.MemoryFixPattern {
	if s == nil {
		return nil
	}
	rows, err := s.db.QueryContext(
		context.Background(),
		`SELECT id, incident_id, proposal_id, title, description, resource, kind, recorded_by, recorded_at
		   FROM memory_fix_patterns
		  ORDER BY recorded_at DESC, id DESC`,
	)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("sql memory fix list failed", "error", err.Error())
		}
		return nil
	}
	defer rows.Close()

	out := make([]model.MemoryFixPattern, 0)
	for rows.Next() {
		var item model.MemoryFixPattern
		var kind string
		if err := rows.Scan(
			&item.ID,
			&item.IncidentID,
			&item.ProposalID,
			&item.Title,
			&item.Description,
			&item.Resource,
			&kind,
			&item.RecordedBy,
			&item.RecordedAt,
		); err != nil {
			continue
		}
		item.Kind = model.RemediationKind(kind)
		out = append(out, cloneFix(item))
	}
	return out
}

func (s *SQLStore) RecordFix(req model.MemoryFixCreateRequest, recordedBy string) (model.MemoryFixPattern, error) {
	if s == nil {
		return model.MemoryFixPattern{}, os.ErrInvalid
	}
	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	resource := strings.TrimSpace(req.Resource)
	if title == "" || description == "" || resource == "" {
		return model.MemoryFixPattern{}, os.ErrInvalid
	}

	nowAt := s.now().UTC().Format(time.RFC3339)
	fix := model.MemoryFixPattern{
		ID:          s.nextID("fix"),
		IncidentID:  strings.TrimSpace(req.IncidentID),
		ProposalID:  strings.TrimSpace(req.ProposalID),
		Title:       title,
		Description: description,
		Resource:    resource,
		Kind:        req.Kind,
		RecordedBy:  defaultString(strings.TrimSpace(recordedBy), "unknown"),
		RecordedAt:  nowAt,
	}
	_, err := s.db.ExecContext(
		context.Background(),
		s.bind(`INSERT INTO memory_fix_patterns (
			id, incident_id, proposal_id, title, description, resource, kind, recorded_by, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		fix.ID,
		fix.IncidentID,
		fix.ProposalID,
		fix.Title,
		fix.Description,
		fix.Resource,
		string(fix.Kind),
		fix.RecordedBy,
		fix.RecordedAt,
	)
	if err != nil {
		return model.MemoryFixPattern{}, err
	}
	return cloneFix(fix), nil
}

func (s *SQLStore) embedRunbook(ctx context.Context, runbook model.MemoryRunbook) ([]float32, error) {
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

func (s *SQLStore) listRunbooks(ctx context.Context) ([]model.MemoryRunbook, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, title, tags_json, description, steps_json, embedding_json, usage_count, created_at, updated_at
		   FROM memory_runbooks
		  ORDER BY updated_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.MemoryRunbook, 0)
	for rows.Next() {
		item, err := scanRunbook(rows)
		if err == nil {
			out = append(out, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func (s *SQLStore) getRunbook(ctx context.Context, id string) (model.MemoryRunbook, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		s.bind(`SELECT id, title, tags_json, description, steps_json, embedding_json, usage_count, created_at, updated_at
		   FROM memory_runbooks
		  WHERE id = ?`),
		id,
	)
	item, err := scanRunbook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MemoryRunbook{}, false, nil
	}
	if err != nil {
		return model.MemoryRunbook{}, false, err
	}
	return item, true, nil
}

func (s *SQLStore) insertRunbook(ctx context.Context, runbook model.MemoryRunbook) error {
	tagsJSON, stepsJSON, embeddingJSON, err := encodeRunbookJSON(runbook)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		s.bind(`INSERT INTO memory_runbooks (
			id, title, tags_json, description, steps_json, embedding_json, usage_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		runbook.ID,
		runbook.Title,
		tagsJSON,
		runbook.Description,
		stepsJSON,
		embeddingJSON,
		runbook.UsageCount,
		runbook.CreatedAt,
		runbook.UpdatedAt,
	)
	return err
}

func (s *SQLStore) updateRunbook(ctx context.Context, runbook model.MemoryRunbook) error {
	tagsJSON, stepsJSON, embeddingJSON, err := encodeRunbookJSON(runbook)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(
		ctx,
		s.bind(`UPDATE memory_runbooks
		    SET title = ?, tags_json = ?, description = ?, steps_json = ?, embedding_json = ?,
		        usage_count = ?, created_at = ?, updated_at = ?
		  WHERE id = ?`),
		runbook.Title,
		tagsJSON,
		runbook.Description,
		stepsJSON,
		embeddingJSON,
		runbook.UsageCount,
		runbook.CreatedAt,
		runbook.UpdatedAt,
		runbook.ID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return os.ErrNotExist
	}
	return nil
}

type runbookScanner interface {
	Scan(dest ...any) error
}

func scanRunbook(row runbookScanner) (model.MemoryRunbook, error) {
	var item model.MemoryRunbook
	var tagsJSON, stepsJSON, embeddingJSON string
	if err := row.Scan(
		&item.ID,
		&item.Title,
		&tagsJSON,
		&item.Description,
		&stepsJSON,
		&embeddingJSON,
		&item.UsageCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return model.MemoryRunbook{}, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
		item.Tags = []string{}
	}
	if err := json.Unmarshal([]byte(stepsJSON), &item.Steps); err != nil {
		item.Steps = []string{}
	}
	if err := json.Unmarshal([]byte(embeddingJSON), &item.Embedding); err != nil {
		item.Embedding = nil
	}
	return cloneRunbook(item), nil
}

func encodeRunbookJSON(runbook model.MemoryRunbook) (string, string, string, error) {
	tagsJSON, err := json.Marshal(runbook.Tags)
	if err != nil {
		return "", "", "", err
	}
	stepsJSON, err := json.Marshal(runbook.Steps)
	if err != nil {
		return "", "", "", err
	}
	embeddingJSON, err := json.Marshal(runbook.Embedding)
	if err != nil {
		return "", "", "", err
	}
	return string(tagsJSON), string(stepsJSON), string(embeddingJSON), nil
}

func (s *SQLStore) bind(query string) string {
	return s.dialect.Bind(query)
}

func (s *SQLStore) nextID(prefix string) string {
	return prefix + "-" + formatCounter(uint64(s.now().UTC().UnixNano())) + "-" + formatCounter(s.seq.Add(1))
}
