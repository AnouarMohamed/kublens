package remediation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"kubelens-backend/internal/model"
)

const (
	DefaultStoreLimit = 100
)

var (
	ErrProposalNotFound      = errors.New("remediation proposal not found")
	ErrProposalNotExecutable = errors.New("proposal must be approved before execution")
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

func (s *Store) SaveProposals(proposals []model.RemediationProposal) []model.RemediationProposal {
	items, err := s.SaveProposalsContext(context.Background(), proposals)
	if err != nil {
		return items
	}
	return items
}

func (s *Store) SaveProposalsContext(
	ctx context.Context,
	proposals []model.RemediationProposal,
) ([]model.RemediationProposal, error) {
	if s == nil || len(proposals) == 0 {
		return nil, nil
	}

	nowAt := s.now().UTC().Format(time.RFC3339)
	out := make([]model.RemediationProposal, 0, len(proposals))
	for _, proposal := range proposals {
		normalized := normalizeProposal(proposal)

		current, found, err := s.findActiveDuplicate(ctx, normalized)
		if err != nil {
			return out, err
		}
		if found {
			if normalized.Reason != "" {
				current.Reason = normalized.Reason
			}
			if normalized.RiskLevel != "" {
				current.RiskLevel = normalized.RiskLevel
			}
			if normalized.DryRunResult != "" {
				current.DryRunResult = normalized.DryRunResult
			}
			if normalized.IncidentID != "" {
				current.IncidentID = normalized.IncidentID
			}
			current.UpdatedAt = nowAt
			if err := s.upsert(ctx, current); err != nil {
				return out, err
			}
			out = append(out, cloneProposal(current))
			continue
		}

		normalized.ID = s.nextID("rem")
		normalized.CreatedAt = nowAt
		normalized.UpdatedAt = nowAt
		if err := s.upsert(ctx, normalized); err != nil {
			return out, err
		}
		out = append(out, cloneProposal(normalized))
	}

	if err := s.trim(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Store) findActiveDuplicate(ctx context.Context, candidate model.RemediationProposal) (model.RemediationProposal, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, kind, status, incident_id, resource, namespace, reason, risk_level, dry_run_result,
		        execution_result, created_at, updated_at, approved_by, approved_at, rejected_by, rejected_at,
		        rejected_reason, executed_by, executed_at
		   FROM remediation_proposals
		  WHERE kind = ?
		    AND LOWER(namespace) = LOWER(?)
		    AND LOWER(resource) = LOWER(?)
		    AND status IN ('proposed', 'approved')
		  ORDER BY created_at DESC, id DESC
		  LIMIT 1`,
		string(candidate.Kind),
		candidate.Namespace,
		candidate.Resource,
	)

	proposal, err := scanProposal(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RemediationProposal{}, false, nil
	}
	if err != nil {
		return model.RemediationProposal{}, false, err
	}
	return proposal, true, nil
}

func (s *Store) List() []model.RemediationProposal {
	items, err := s.ListContext(context.Background())
	if err != nil {
		return nil
	}
	return items
}

func (s *Store) ListContext(ctx context.Context) ([]model.RemediationProposal, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, kind, status, incident_id, resource, namespace, reason, risk_level, dry_run_result,
		        execution_result, created_at, updated_at, approved_by, approved_at, rejected_by, rejected_at,
		        rejected_reason, executed_by, executed_at
		   FROM remediation_proposals
		  ORDER BY created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.RemediationProposal, 0, s.maxItems)
	for rows.Next() {
		item, decodeErr := scanProposal(rows)
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

func (s *Store) Get(id string) (model.RemediationProposal, bool) {
	item, ok, err := s.GetContext(context.Background(), id)
	if err != nil {
		return model.RemediationProposal{}, false
	}
	return item, ok
}

func (s *Store) GetContext(ctx context.Context, id string) (model.RemediationProposal, bool, error) {
	if s == nil {
		return model.RemediationProposal{}, false, nil
	}

	needle := strings.TrimSpace(id)
	if needle == "" {
		return model.RemediationProposal{}, false, nil
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, kind, status, incident_id, resource, namespace, reason, risk_level, dry_run_result,
		        execution_result, created_at, updated_at, approved_by, approved_at, rejected_by, rejected_at,
		        rejected_reason, executed_by, executed_at
		   FROM remediation_proposals
		  WHERE id = ?`,
		needle,
	)

	item, err := scanProposal(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RemediationProposal{}, false, nil
		}
		return model.RemediationProposal{}, false, err
	}
	return item, true, nil
}

func (s *Store) Approve(id string, user string) (model.RemediationProposal, error) {
	return s.ApproveContext(context.Background(), id, user)
}

func (s *Store) ApproveContext(ctx context.Context, id string, user string) (model.RemediationProposal, error) {
	return s.updateWithUser(id, user, func(proposal *model.RemediationProposal, nowAt string, actor string) error {
		if proposal.Status == "executed" || proposal.Status == "rejected" {
			return fmt.Errorf("proposal already %s", proposal.Status)
		}
		proposal.Status = "approved"
		proposal.ApprovedBy = actor
		proposal.ApprovedAt = nowAt
		return nil
	}, ctx)
}

func (s *Store) Reject(id string, user string, reason string) (model.RemediationProposal, error) {
	return s.RejectContext(context.Background(), id, user, reason)
}

func (s *Store) RejectContext(ctx context.Context, id string, user string, reason string) (model.RemediationProposal, error) {
	return s.updateWithUser(id, user, func(proposal *model.RemediationProposal, nowAt string, actor string) error {
		if proposal.Status == "executed" {
			return errors.New("executed proposals cannot be rejected")
		}
		proposal.Status = "rejected"
		proposal.RejectedBy = actor
		proposal.RejectedAt = nowAt
		proposal.RejectedReason = strings.TrimSpace(reason)
		return nil
	}, ctx)
}

func (s *Store) MarkExecuted(id string, user string, result string) (model.RemediationProposal, error) {
	return s.MarkExecutedContext(context.Background(), id, user, result)
}

func (s *Store) MarkExecutedContext(
	ctx context.Context,
	id string,
	user string,
	result string,
) (model.RemediationProposal, error) {
	return s.updateWithUser(id, user, func(proposal *model.RemediationProposal, nowAt string, actor string) error {
		if proposal.Status != "approved" {
			return ErrProposalNotExecutable
		}
		proposal.Status = "executed"
		proposal.ExecutedBy = actor
		proposal.ExecutedAt = nowAt
		proposal.ExecutionResult = strings.TrimSpace(result)
		return nil
	}, ctx)
}

func (s *Store) updateWithUser(
	id string,
	user string,
	mutate func(proposal *model.RemediationProposal, nowAt string, actor string) error,
	ctx context.Context,
) (model.RemediationProposal, error) {
	if s == nil {
		return model.RemediationProposal{}, ErrProposalNotFound
	}

	needle := strings.TrimSpace(id)
	if needle == "" {
		return model.RemediationProposal{}, ErrProposalNotFound
	}
	actor := strings.TrimSpace(user)
	if actor == "" {
		actor = "unknown"
	}

	proposal, ok, err := s.GetContext(ctx, needle)
	if err != nil {
		return model.RemediationProposal{}, err
	}
	if !ok {
		return model.RemediationProposal{}, ErrProposalNotFound
	}

	nowAt := s.now().UTC().Format(time.RFC3339)
	if err := mutate(&proposal, nowAt, actor); err != nil {
		return model.RemediationProposal{}, err
	}
	proposal.UpdatedAt = nowAt
	if err := s.upsert(ctx, proposal); err != nil {
		return model.RemediationProposal{}, err
	}
	return cloneProposal(proposal), nil
}

func (s *Store) upsert(ctx context.Context, proposal model.RemediationProposal) error {
	if s == nil {
		return ErrProposalNotFound
	}

	createdAt, err := s.lookupCreatedAt(ctx, proposal.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(createdAt) == "" {
		createdAt = fallbackString(proposal.CreatedAt, s.now().UTC().Format(time.RFC3339))
	}
	updatedAt := fallbackString(proposal.UpdatedAt, s.now().UTC().Format(time.RFC3339))

	_, err = s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO remediation_proposals (
			id, kind, status, incident_id, resource, namespace, reason, risk_level, dry_run_result,
			execution_result, approved_by, approved_at, rejected_by, rejected_at, rejected_reason,
			executed_by, executed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proposal.ID,
		string(proposal.Kind),
		proposal.Status,
		proposal.IncidentID,
		proposal.Resource,
		proposal.Namespace,
		proposal.Reason,
		proposal.RiskLevel,
		proposal.DryRunResult,
		proposal.ExecutionResult,
		proposal.ApprovedBy,
		proposal.ApprovedAt,
		proposal.RejectedBy,
		proposal.RejectedAt,
		proposal.RejectedReason,
		proposal.ExecutedBy,
		proposal.ExecutedAt,
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
		`DELETE FROM remediation_proposals
		  WHERE id IN (
				SELECT id
				  FROM remediation_proposals
				 ORDER BY created_at DESC, id DESC
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
	err := s.db.QueryRowContext(ctx, `SELECT created_at FROM remediation_proposals WHERE id = ?`, id).Scan(&createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return createdAt, err
}

func (s *Store) nextID(prefix string) string {
	seq := s.seq.Add(1)
	return fmt.Sprintf("%s-%d-%d", prefix, s.now().UTC().UnixNano(), seq)
}

func normalizeProposal(in model.RemediationProposal) model.RemediationProposal {
	out := in
	out.ID = strings.TrimSpace(in.ID)
	out.Status = fallbackString(strings.TrimSpace(in.Status), "proposed")
	out.IncidentID = strings.TrimSpace(in.IncidentID)
	out.Namespace = strings.TrimSpace(in.Namespace)
	out.Resource = strings.TrimSpace(in.Resource)
	out.Reason = strings.TrimSpace(in.Reason)
	out.RiskLevel = strings.TrimSpace(in.RiskLevel)
	out.DryRunResult = strings.TrimSpace(in.DryRunResult)
	out.ExecutionResult = strings.TrimSpace(in.ExecutionResult)
	out.CreatedAt = strings.TrimSpace(in.CreatedAt)
	out.UpdatedAt = strings.TrimSpace(in.UpdatedAt)
	out.ApprovedBy = strings.TrimSpace(in.ApprovedBy)
	out.ApprovedAt = strings.TrimSpace(in.ApprovedAt)
	out.RejectedBy = strings.TrimSpace(in.RejectedBy)
	out.RejectedAt = strings.TrimSpace(in.RejectedAt)
	out.RejectedReason = strings.TrimSpace(in.RejectedReason)
	out.ExecutedBy = strings.TrimSpace(in.ExecutedBy)
	out.ExecutedAt = strings.TrimSpace(in.ExecutedAt)
	return out
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func cloneProposal(in model.RemediationProposal) model.RemediationProposal {
	out := in
	return out
}

type proposalScanner interface {
	Scan(dest ...any) error
}

func scanProposal(scanner proposalScanner) (model.RemediationProposal, error) {
	var (
		proposal model.RemediationProposal
		kind     string
	)

	if err := scanner.Scan(
		&proposal.ID,
		&kind,
		&proposal.Status,
		&proposal.IncidentID,
		&proposal.Resource,
		&proposal.Namespace,
		&proposal.Reason,
		&proposal.RiskLevel,
		&proposal.DryRunResult,
		&proposal.ExecutionResult,
		&proposal.CreatedAt,
		&proposal.UpdatedAt,
		&proposal.ApprovedBy,
		&proposal.ApprovedAt,
		&proposal.RejectedBy,
		&proposal.RejectedAt,
		&proposal.RejectedReason,
		&proposal.ExecutedBy,
		&proposal.ExecutedAt,
	); err != nil {
		return model.RemediationProposal{}, err
	}

	proposal.Kind = model.RemediationKind(kind)
	return cloneProposal(proposal), nil
}
