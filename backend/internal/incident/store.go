package incident

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"kubelens-backend/internal/model"
)

const (
	DefaultStoreLimit = 50
)

var (
	ErrIncidentNotFound = errors.New("incident not found")
	ErrStepNotFound     = errors.New("runbook step not found")
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

func (s *Store) Create(incident model.Incident) model.Incident {
	created, err := s.CreateContext(context.Background(), incident)
	if err != nil {
		return created
	}
	return created
}

func (s *Store) CreateContext(ctx context.Context, incident model.Incident) (model.Incident, error) {
	if s == nil {
		return incident, ErrIncidentNotFound
	}

	created := cloneIncident(incident)
	if strings.TrimSpace(created.ID) == "" {
		created.ID = s.nextID("inc")
	}
	created.Status = model.IncidentStatusOpen
	if strings.TrimSpace(created.OpenedAt) == "" {
		created.OpenedAt = s.now().UTC().Format(time.RFC3339)
	}
	created.ResolvedAt = ""
	if created.AssociatedRemediationIDs == nil {
		created.AssociatedRemediationIDs = []string{}
	}

	if err := s.save(ctx, created); err != nil {
		return cloneIncident(created), err
	}
	return cloneIncident(created), nil
}

func (s *Store) List() []model.Incident {
	items, err := s.ListContext(context.Background())
	if err != nil {
		return nil
	}
	return items
}

func (s *Store) ListContext(ctx context.Context) ([]model.Incident, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, title, severity, status, summary, opened_at, resolved_at, timeline_json, runbook_json,
		        affected_resources_json, associated_remediation_ids_json
		   FROM incidents
		  ORDER BY opened_at DESC, created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Incident, 0, s.maxItems)
	for rows.Next() {
		item, decodeErr := scanIncident(rows)
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

func (s *Store) Get(id string) (model.Incident, bool) {
	item, ok, err := s.GetContext(context.Background(), id)
	if err != nil {
		return model.Incident{}, false
	}
	return item, ok
}

func (s *Store) GetContext(ctx context.Context, id string) (model.Incident, bool, error) {
	if s == nil {
		return model.Incident{}, false, nil
	}

	needle := strings.TrimSpace(id)
	if needle == "" {
		return model.Incident{}, false, nil
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, title, severity, status, summary, opened_at, resolved_at, timeline_json, runbook_json,
		        affected_resources_json, associated_remediation_ids_json
		   FROM incidents
		  WHERE id = ?`,
		needle,
	)

	item, err := scanIncident(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Incident{}, false, nil
		}
		return model.Incident{}, false, err
	}
	return item, true, nil
}

func (s *Store) PatchStepStatus(id string, stepID string, target model.RunbookStepStatus) (model.Incident, error) {
	return s.PatchStepStatusContext(context.Background(), id, stepID, target)
}

func (s *Store) PatchStepStatusContext(
	ctx context.Context,
	id string,
	stepID string,
	target model.RunbookStepStatus,
) (model.Incident, error) {
	if s == nil {
		return model.Incident{}, ErrIncidentNotFound
	}

	incidentID := strings.TrimSpace(id)
	runbookStepID := strings.TrimSpace(stepID)
	if incidentID == "" || runbookStepID == "" {
		return model.Incident{}, ErrIncidentNotFound
	}

	incident, ok, err := s.GetContext(ctx, incidentID)
	if err != nil {
		return model.Incident{}, err
	}
	if !ok {
		return model.Incident{}, ErrIncidentNotFound
	}

	for i := range incident.Runbook {
		step := &incident.Runbook[i]
		if step.ID != runbookStepID {
			continue
		}

		if step.Mandatory && target == model.RunbookStepStatusSkipped {
			return model.Incident{}, errors.New("final verification step cannot be skipped")
		}
		if err := validateStatusTransition(step.Status, target); err != nil {
			return model.Incident{}, err
		}

		step.Status = target
		nowAt := s.now().UTC().Format(time.RFC3339)
		incident.Timeline = append(incident.Timeline, model.TimelineEntry{
			Timestamp: nowAt,
			Kind:      model.TimelineEntryKindAction,
			Source:    "incident-commander",
			Summary:   fmt.Sprintf("Runbook step %s moved to %s", step.ID, step.Status),
			Resource:  "",
			Severity:  "info",
		})
		sort.SliceStable(incident.Timeline, func(i, j int) bool {
			return incident.Timeline[i].Timestamp < incident.Timeline[j].Timestamp
		})
		if err := s.save(ctx, incident); err != nil {
			return model.Incident{}, err
		}
		return cloneIncident(incident), nil
	}

	return model.Incident{}, ErrStepNotFound
}

func (s *Store) Resolve(id string) (model.Incident, error) {
	return s.ResolveContext(context.Background(), id)
}

func (s *Store) ResolveContext(ctx context.Context, id string) (model.Incident, error) {
	if s == nil {
		return model.Incident{}, ErrIncidentNotFound
	}

	incidentID := strings.TrimSpace(id)
	if incidentID == "" {
		return model.Incident{}, ErrIncidentNotFound
	}

	incident, ok, err := s.GetContext(ctx, incidentID)
	if err != nil {
		return model.Incident{}, err
	}
	if !ok {
		return model.Incident{}, ErrIncidentNotFound
	}
	if incident.Status == model.IncidentStatusResolved {
		return incident, nil
	}
	if !canResolveIncident(incident) {
		return model.Incident{}, errors.New("incident cannot be resolved: all runbook steps must be done or skipped")
	}

	nowAt := s.now().UTC().Format(time.RFC3339)
	incident.Status = model.IncidentStatusResolved
	incident.ResolvedAt = nowAt
	incident.Timeline = append(incident.Timeline, model.TimelineEntry{
		Timestamp: nowAt,
		Kind:      model.TimelineEntryKindAction,
		Source:    "incident-commander",
		Summary:   "Incident resolved",
		Resource:  "",
		Severity:  "info",
	})
	sort.SliceStable(incident.Timeline, func(i, j int) bool {
		return incident.Timeline[i].Timestamp < incident.Timeline[j].Timestamp
	})

	if err := s.save(ctx, incident); err != nil {
		return model.Incident{}, err
	}
	return cloneIncident(incident), nil
}

func canResolveIncident(incident model.Incident) bool {
	for _, step := range incident.Runbook {
		if step.Mandatory {
			if step.Status != model.RunbookStepStatusDone {
				return false
			}
			continue
		}
		if step.Status != model.RunbookStepStatusDone && step.Status != model.RunbookStepStatusSkipped {
			return false
		}
	}
	return true
}

func (s *Store) AssociateRemediation(incidentID string, proposalID string) error {
	return s.AssociateRemediationContext(context.Background(), incidentID, proposalID)
}

func (s *Store) AssociateRemediationContext(ctx context.Context, incidentID string, proposalID string) error {
	if s == nil {
		return ErrIncidentNotFound
	}

	incID := strings.TrimSpace(incidentID)
	propID := strings.TrimSpace(proposalID)
	if incID == "" || propID == "" {
		return ErrIncidentNotFound
	}

	incident, ok, err := s.GetContext(ctx, incID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrIncidentNotFound
	}

	for _, id := range incident.AssociatedRemediationIDs {
		if id == propID {
			return nil
		}
	}
	incident.AssociatedRemediationIDs = append(incident.AssociatedRemediationIDs, propID)
	sort.Strings(incident.AssociatedRemediationIDs)
	return s.save(ctx, incident)
}

func (s *Store) save(ctx context.Context, incident model.Incident) error {
	if s == nil {
		return ErrIncidentNotFound
	}

	createdAt, err := s.lookupCreatedAt(ctx, incident.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(createdAt) == "" {
		createdAt = fallbackTime(incident.OpenedAt, s.now())
	}
	nowAt := s.now().UTC().Format(time.RFC3339)

	timelineJSON, err := marshalJSON(incident.Timeline)
	if err != nil {
		return err
	}
	runbookJSON, err := marshalJSON(incident.Runbook)
	if err != nil {
		return err
	}
	affectedJSON, err := marshalJSON(incident.AffectedResources)
	if err != nil {
		return err
	}
	associatedJSON, err := marshalJSON(incident.AssociatedRemediationIDs)
	if err != nil {
		return err
	}

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO incidents (
			id, title, severity, status, summary, opened_at, resolved_at, timeline_json, runbook_json,
			affected_resources_json, associated_remediation_ids_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		incident.ID,
		incident.Title,
		incident.Severity,
		string(incident.Status),
		incident.Summary,
		incident.OpenedAt,
		incident.ResolvedAt,
		timelineJSON,
		runbookJSON,
		affectedJSON,
		associatedJSON,
		createdAt,
		nowAt,
	); err != nil {
		return err
	}

	return s.trim(ctx)
}

func (s *Store) trim(ctx context.Context) error {
	if s.maxItems <= 0 {
		return nil
	}

	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM incidents
		  WHERE id IN (
				SELECT id
				  FROM incidents
				 ORDER BY opened_at DESC, created_at DESC, id DESC
				 LIMIT -1 OFFSET ?
		  )`,
		s.maxItems,
	)
	return err
}

func (s *Store) lookupCreatedAt(ctx context.Context, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", nil
	}

	var createdAt string
	err := s.db.QueryRowContext(ctx, `SELECT created_at FROM incidents WHERE id = ?`, id).Scan(&createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return createdAt, err
}

func (s *Store) nextID(prefix string) string {
	seq := s.seq.Add(1)
	return fmt.Sprintf("%s-%d-%d", prefix, s.now().UTC().UnixNano(), seq)
}

func validateStatusTransition(from model.RunbookStepStatus, to model.RunbookStepStatus) error {
	valid := false
	switch from {
	case model.RunbookStepStatusPending:
		valid = to == model.RunbookStepStatusInProgress || to == model.RunbookStepStatusSkipped
	case model.RunbookStepStatusInProgress:
		valid = to == model.RunbookStepStatusDone || to == model.RunbookStepStatusSkipped
	default:
		valid = false
	}

	if valid {
		return nil
	}
	return fmt.Errorf("invalid status transition: %s → %s", from, to)
}

func cloneIncident(in model.Incident) model.Incident {
	out := in
	out.Timeline = append([]model.TimelineEntry(nil), in.Timeline...)
	out.Runbook = append([]model.RunbookStep(nil), in.Runbook...)
	out.AffectedResources = append([]string(nil), in.AffectedResources...)
	out.AssociatedRemediationIDs = append([]string(nil), in.AssociatedRemediationIDs...)
	return out
}

type incidentScanner interface {
	Scan(dest ...any) error
}

func scanIncident(scanner incidentScanner) (model.Incident, error) {
	var (
		incident       model.Incident
		status         string
		timelineJSON   string
		runbookJSON    string
		affectedJSON   string
		associatedJSON string
	)

	if err := scanner.Scan(
		&incident.ID,
		&incident.Title,
		&incident.Severity,
		&status,
		&incident.Summary,
		&incident.OpenedAt,
		&incident.ResolvedAt,
		&timelineJSON,
		&runbookJSON,
		&affectedJSON,
		&associatedJSON,
	); err != nil {
		return model.Incident{}, err
	}

	incident.Status = model.IncidentStatus(status)
	if err := unmarshalJSON(timelineJSON, &incident.Timeline); err != nil {
		return model.Incident{}, err
	}
	if err := unmarshalJSON(runbookJSON, &incident.Runbook); err != nil {
		return model.Incident{}, err
	}
	if err := unmarshalJSON(affectedJSON, &incident.AffectedResources); err != nil {
		return model.Incident{}, err
	}
	if err := unmarshalJSON(associatedJSON, &incident.AssociatedRemediationIDs); err != nil {
		return model.Incident{}, err
	}

	return cloneIncident(incident), nil
}

func marshalJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalJSON(raw string, target any) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "[]"
	}
	return json.Unmarshal([]byte(trimmed), target)
}

func fallbackTime(value string, now time.Time) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return now.UTC().Format(time.RFC3339)
}
