package postmortem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"kubelens-backend/internal/model"
)

const DefaultStoreLimit = 50

var (
	ErrPostmortemNotFound = errors.New("postmortem not found")
	ErrPostmortemExists   = errors.New("postmortem already exists for incident")
)

type Store struct {
	db       *sql.DB
	maxItems int
	now      func() time.Time
	seq      atomic.Uint64
}

func NewStore(db *sql.DB, maxItems int, now func() time.Time) *Store {
	if db == nil {
		return nil
	}
	if maxItems <= 0 {
		maxItems = DefaultStoreLimit
	}
	clock := now
	if clock == nil {
		clock = time.Now
	}

	store := &Store{
		db:       db,
		maxItems: maxItems,
		now:      clock,
	}
	store.seq.Store(uint64(clock().UTC().UnixNano()))
	return store
}

func (s *Store) Create(postmortem model.Postmortem) (model.Postmortem, error) {
	return s.CreateContext(context.Background(), postmortem)
}

func (s *Store) CreateContext(ctx context.Context, postmortem model.Postmortem) (model.Postmortem, error) {
	if s == nil {
		return model.Postmortem{}, ErrPostmortemNotFound
	}

	created := clonePostmortem(postmortem)
	created.IncidentID = strings.TrimSpace(created.IncidentID)
	if created.IncidentID == "" {
		return model.Postmortem{}, ErrPostmortemNotFound
	}
	if strings.TrimSpace(created.ID) == "" {
		created.ID = s.nextID("pm")
	}
	if strings.TrimSpace(created.GeneratedAt) == "" {
		created.GeneratedAt = s.now().UTC().Format(time.RFC3339)
	}

	if err := s.insert(ctx, created); err != nil {
		if isPostmortemExistsError(err) {
			if existing, ok, lookupErr := s.GetByIncidentIDContext(ctx, created.IncidentID); lookupErr == nil && ok {
				return existing, fmt.Errorf("%w: %s", ErrPostmortemExists, existing.ID)
			}
			return model.Postmortem{}, ErrPostmortemExists
		}
		return model.Postmortem{}, err
	}
	if err := s.trim(ctx); err != nil {
		return model.Postmortem{}, err
	}
	return clonePostmortem(created), nil
}

func (s *Store) List() []model.Postmortem {
	items, err := s.ListContext(context.Background())
	if err != nil {
		return nil
	}
	return items
}

func (s *Store) ListContext(ctx context.Context) ([]model.Postmortem, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, incident_id, incident_title, severity, opened_at, resolved_at, duration, generated_at,
		        method, root_cause, impact, prevention, timeline_markdown, runbook_markdown, timeline_json, runbook_json
		   FROM postmortems
		  ORDER BY generated_at DESC, created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Postmortem, 0, s.maxItems)
	for rows.Next() {
		item, decodeErr := scanPostmortem(rows)
		if decodeErr != nil {
			return nil, decodeErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) Get(id string) (model.Postmortem, bool) {
	item, ok, err := s.GetContext(context.Background(), id)
	if err != nil {
		return model.Postmortem{}, false
	}
	return item, ok
}

func (s *Store) GetContext(ctx context.Context, id string) (model.Postmortem, bool, error) {
	if s == nil {
		return model.Postmortem{}, false, nil
	}
	needle := strings.TrimSpace(id)
	if needle == "" {
		return model.Postmortem{}, false, nil
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, incident_id, incident_title, severity, opened_at, resolved_at, duration, generated_at,
		        method, root_cause, impact, prevention, timeline_markdown, runbook_markdown, timeline_json, runbook_json
		   FROM postmortems
		  WHERE id = ?`,
		needle,
	)

	item, err := scanPostmortem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Postmortem{}, false, nil
		}
		return model.Postmortem{}, false, err
	}
	return item, true, nil
}

func (s *Store) GetByIncidentID(incidentID string) (model.Postmortem, bool) {
	item, ok, err := s.GetByIncidentIDContext(context.Background(), incidentID)
	if err != nil {
		return model.Postmortem{}, false
	}
	return item, ok
}

func (s *Store) GetByIncidentIDContext(
	ctx context.Context,
	incidentID string,
) (model.Postmortem, bool, error) {
	if s == nil {
		return model.Postmortem{}, false, nil
	}
	needle := strings.TrimSpace(incidentID)
	if needle == "" {
		return model.Postmortem{}, false, nil
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, incident_id, incident_title, severity, opened_at, resolved_at, duration, generated_at,
		        method, root_cause, impact, prevention, timeline_markdown, runbook_markdown, timeline_json, runbook_json
		   FROM postmortems
		  WHERE incident_id = ?`,
		needle,
	)

	item, err := scanPostmortem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Postmortem{}, false, nil
		}
		return model.Postmortem{}, false, err
	}
	return item, true, nil
}

func (s *Store) insert(ctx context.Context, postmortem model.Postmortem) error {
	timelineJSON, err := marshalPostmortemJSON(postmortem.Timeline)
	if err != nil {
		return err
	}
	runbookJSON, err := marshalPostmortemJSON(postmortem.Runbook)
	if err != nil {
		return err
	}

	createdAt := fallbackPostmortemTime(postmortem.GeneratedAt, s.now())
	updatedAt := s.now().UTC().Format(time.RFC3339)

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO postmortems (
			id, incident_id, incident_title, severity, opened_at, resolved_at, duration, generated_at,
			method, root_cause, impact, prevention, timeline_markdown, runbook_markdown, timeline_json,
			runbook_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		postmortem.ID,
		postmortem.IncidentID,
		postmortem.IncidentTitle,
		postmortem.Severity,
		postmortem.OpenedAt,
		postmortem.ResolvedAt,
		postmortem.Duration,
		postmortem.GeneratedAt,
		string(postmortem.Method),
		postmortem.RootCause,
		postmortem.Impact,
		postmortem.Prevention,
		postmortem.TimelineMarkdown,
		postmortem.RunbookMarkdown,
		timelineJSON,
		runbookJSON,
		createdAt,
		updatedAt,
	)
	return err
}

func (s *Store) trim(ctx context.Context) error {
	if s.maxItems <= 0 {
		return nil
	}

	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM postmortems
		  WHERE id IN (
				SELECT id
				  FROM postmortems
				 ORDER BY generated_at DESC, created_at DESC, id DESC
				 LIMIT -1 OFFSET ?
		  )`,
		s.maxItems,
	)
	return err
}

func (s *Store) nextID(prefix string) string {
	seq := s.seq.Add(1)
	return fmt.Sprintf("%s-%d-%d", prefix, s.now().UTC().UnixNano(), seq)
}

func clonePostmortem(in model.Postmortem) model.Postmortem {
	out := in
	out.Timeline = append([]model.TimelineEntry(nil), in.Timeline...)
	out.Runbook = append([]model.RunbookStep(nil), in.Runbook...)
	return out
}

type postmortemScanner interface {
	Scan(dest ...any) error
}

func scanPostmortem(scanner postmortemScanner) (model.Postmortem, error) {
	var (
		postmortem   model.Postmortem
		method       string
		timelineJSON string
		runbookJSON  string
	)

	if err := scanner.Scan(
		&postmortem.ID,
		&postmortem.IncidentID,
		&postmortem.IncidentTitle,
		&postmortem.Severity,
		&postmortem.OpenedAt,
		&postmortem.ResolvedAt,
		&postmortem.Duration,
		&postmortem.GeneratedAt,
		&method,
		&postmortem.RootCause,
		&postmortem.Impact,
		&postmortem.Prevention,
		&postmortem.TimelineMarkdown,
		&postmortem.RunbookMarkdown,
		&timelineJSON,
		&runbookJSON,
	); err != nil {
		return model.Postmortem{}, err
	}

	postmortem.Method = model.PostmortemMethod(method)
	if err := unmarshalPostmortemJSON(timelineJSON, &postmortem.Timeline); err != nil {
		return model.Postmortem{}, err
	}
	if err := unmarshalPostmortemJSON(runbookJSON, &postmortem.Runbook); err != nil {
		return model.Postmortem{}, err
	}

	return clonePostmortem(postmortem), nil
}

func marshalPostmortemJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalPostmortemJSON(raw string, target any) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "[]"
	}
	return json.Unmarshal([]byte(trimmed), target)
}

func fallbackPostmortemTime(value string, now time.Time) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return now.UTC().Format(time.RFC3339)
}

func isPostmortemExistsError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed: postmortems.incident_id")
}
