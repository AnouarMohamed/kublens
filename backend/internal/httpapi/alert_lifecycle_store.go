package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

const defaultAlertLifecycleLimit = 2000

var errAlertLifecycleInvalid = errors.New("invalid alert lifecycle payload")

type alertLifecycleStore struct {
	db       *sql.DB
	now      func() time.Time
	maxItems int
}

func newAlertLifecycleStore(db *sql.DB, maxItems int, now func() time.Time) *alertLifecycleStore {
	if db == nil {
		return nil
	}

	limit := maxItems
	if limit <= 0 {
		limit = defaultAlertLifecycleLimit
	}
	clock := now
	if clock == nil {
		clock = time.Now
	}
	return &alertLifecycleStore{
		db:       db,
		now:      clock,
		maxItems: limit,
	}
}

func (s *alertLifecycleStore) List(ctx context.Context) []model.NodeAlertLifecycle {
	if s == nil {
		return nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT alert_id, node, rule, status, note, snoozed_until, updated_at, updated_by
		   FROM alert_lifecycle
		  ORDER BY updated_at DESC, alert_id DESC`,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	nowAt := s.now().UTC()
	out := make([]model.NodeAlertLifecycle, 0, s.maxItems)
	for rows.Next() {
		item, scanErr := scanAlertLifecycle(rows)
		if scanErr != nil {
			return nil
		}
		normalized := normalizeLifecycleExpiry(item, nowAt)
		if normalized != item {
			if err := s.persistNormalized(ctx, normalized); err != nil {
				return nil
			}
		}
		out = append(out, normalized)
	}
	if err := rows.Err(); err != nil {
		return nil
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

func (s *alertLifecycleStore) Upsert(
	ctx context.Context,
	req model.NodeAlertLifecycleUpdateRequest,
	actor string,
) (model.NodeAlertLifecycle, error) {
	if s == nil {
		return model.NodeAlertLifecycle{}, errAlertLifecycleInvalid
	}

	id := strings.TrimSpace(req.ID)
	node := strings.TrimSpace(req.Node)
	rule := strings.TrimSpace(req.Rule)
	note := strings.TrimSpace(req.Note)
	status := normalizeLifecycleStatus(req.Status)

	if id == "" || node == "" || rule == "" || status == "" {
		return model.NodeAlertLifecycle{}, errAlertLifecycleInvalid
	}

	updatedBy := strings.TrimSpace(actor)
	if updatedBy == "" {
		updatedBy = "unknown"
	}

	nowAt := s.now().UTC()
	out := model.NodeAlertLifecycle{
		ID:        id,
		Node:      node,
		Rule:      rule,
		Status:    status,
		Note:      note,
		UpdatedAt: nowAt.Format(time.RFC3339),
		UpdatedBy: updatedBy,
	}

	if status == model.NodeAlertStatusSnoozed {
		if req.SnoozeMinutes <= 0 || req.SnoozeMinutes > 24*60 {
			return model.NodeAlertLifecycle{}, errAlertLifecycleInvalid
		}
		out.SnoozedUntil = nowAt.Add(time.Duration(req.SnoozeMinutes) * time.Minute).Format(time.RFC3339)
	}

	createdAt, err := s.lookupCreatedAt(ctx, id)
	if err != nil {
		return model.NodeAlertLifecycle{}, err
	}
	if strings.TrimSpace(createdAt) == "" {
		createdAt = out.UpdatedAt
	}

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO alert_lifecycle (
			alert_id, node, rule, status, note, snoozed_until, updated_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		out.ID,
		out.Node,
		out.Rule,
		string(out.Status),
		out.Note,
		out.SnoozedUntil,
		out.UpdatedBy,
		createdAt,
		out.UpdatedAt,
	); err != nil {
		return model.NodeAlertLifecycle{}, err
	}

	if err := s.trim(ctx); err != nil {
		return model.NodeAlertLifecycle{}, err
	}

	return out, nil
}

func (s *alertLifecycleStore) trim(ctx context.Context) error {
	if s.maxItems <= 0 {
		return nil
	}

	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM alert_lifecycle
		  WHERE alert_id IN (
				SELECT alert_id
				  FROM alert_lifecycle
				 ORDER BY updated_at DESC, alert_id DESC
				 LIMIT -1 OFFSET ?
		  )`,
		s.maxItems,
	)
	return err
}

func (s *alertLifecycleStore) lookupCreatedAt(ctx context.Context, id string) (string, error) {
	var createdAt string
	err := s.db.QueryRowContext(ctx, `SELECT created_at FROM alert_lifecycle WHERE alert_id = ?`, id).Scan(&createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return createdAt, err
}

func (s *alertLifecycleStore) persistNormalized(ctx context.Context, item model.NodeAlertLifecycle) error {
	createdAt, err := s.lookupCreatedAt(ctx, item.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(createdAt) == "" {
		createdAt = item.UpdatedAt
	}

	_, err = s.db.ExecContext(
		ctx,
		`UPDATE alert_lifecycle
		    SET status = ?, note = ?, snoozed_until = ?, updated_at = ?, updated_by = ?, created_at = ?
		  WHERE alert_id = ?`,
		string(item.Status),
		item.Note,
		item.SnoozedUntil,
		item.UpdatedAt,
		item.UpdatedBy,
		createdAt,
		item.ID,
	)
	return err
}

func normalizeLifecycleStatus(raw model.NodeAlertLifecycleStatus) model.NodeAlertLifecycleStatus {
	switch strings.ToLower(strings.TrimSpace(string(raw))) {
	case string(model.NodeAlertStatusActive):
		return model.NodeAlertStatusActive
	case string(model.NodeAlertStatusAcknowledged):
		return model.NodeAlertStatusAcknowledged
	case string(model.NodeAlertStatusSnoozed):
		return model.NodeAlertStatusSnoozed
	case string(model.NodeAlertStatusDismissed):
		return model.NodeAlertStatusDismissed
	default:
		return ""
	}
}

func normalizeLifecycleExpiry(item model.NodeAlertLifecycle, nowAt time.Time) model.NodeAlertLifecycle {
	if item.Status != model.NodeAlertStatusSnoozed || strings.TrimSpace(item.SnoozedUntil) == "" {
		return item
	}

	until, err := time.Parse(time.RFC3339, item.SnoozedUntil)
	if err != nil {
		item.Status = model.NodeAlertStatusActive
		item.SnoozedUntil = ""
		item.UpdatedAt = nowAt.Format(time.RFC3339)
		return item
	}
	if nowAt.After(until) || nowAt.Equal(until) {
		item.Status = model.NodeAlertStatusActive
		item.SnoozedUntil = ""
		item.UpdatedAt = nowAt.Format(time.RFC3339)
	}
	return item
}

type alertLifecycleScanner interface {
	Scan(dest ...any) error
}

func scanAlertLifecycle(scanner alertLifecycleScanner) (model.NodeAlertLifecycle, error) {
	var (
		item   model.NodeAlertLifecycle
		status string
	)

	if err := scanner.Scan(
		&item.ID,
		&item.Node,
		&item.Rule,
		&status,
		&item.Note,
		&item.SnoozedUntil,
		&item.UpdatedAt,
		&item.UpdatedBy,
	); err != nil {
		return model.NodeAlertLifecycle{}, err
	}

	item.Status = model.NodeAlertLifecycleStatus(status)
	return item, nil
}
