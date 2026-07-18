package remediation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

func (s *Store) GetGitOpsArtifact(proposalID string) (model.RemediationGitOpsArtifact, bool) {
	item, ok, err := s.GetGitOpsArtifactContext(context.Background(), proposalID)
	if err != nil {
		return model.RemediationGitOpsArtifact{}, false
	}
	return item, ok
}

func (s *Store) GetGitOpsArtifactContext(
	ctx context.Context,
	proposalID string,
) (model.RemediationGitOpsArtifact, bool, error) {
	if s == nil {
		return model.RemediationGitOpsArtifact{}, false, nil
	}

	needle := strings.TrimSpace(proposalID)
	if needle == "" {
		return model.RemediationGitOpsArtifact{}, false, nil
	}

	row := s.db.QueryRowContext(
		ctx,
		s.bind(`SELECT proposal_id, support_level, strategy, summary, branch_name, pr_title, commit_message,
		        target_path, target_kind, target_namespace, target_name, format, artifact_body,
		        instructions_json, generated_by, generated_at, updated_at
		   FROM remediation_gitops_artifacts
		  WHERE proposal_id = ?`),
		needle,
	)

	item, err := scanGitOpsArtifact(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RemediationGitOpsArtifact{}, false, nil
	}
	if err != nil {
		return model.RemediationGitOpsArtifact{}, false, err
	}
	return item, true, nil
}

func (s *Store) UpsertGitOpsArtifact(
	proposalID string,
	artifact model.GitOpsArtifact,
	generatedBy string,
) (model.RemediationGitOpsArtifact, error) {
	return s.UpsertGitOpsArtifactContext(context.Background(), proposalID, artifact, generatedBy)
}

func (s *Store) UpsertGitOpsArtifactContext(
	ctx context.Context,
	proposalID string,
	artifact model.GitOpsArtifact,
	generatedBy string,
) (model.RemediationGitOpsArtifact, error) {
	if s == nil {
		return model.RemediationGitOpsArtifact{}, ErrProposalNotFound
	}

	needle := strings.TrimSpace(proposalID)
	if needle == "" {
		return model.RemediationGitOpsArtifact{}, ErrProposalNotFound
	}
	if _, ok, err := s.GetContext(ctx, needle); err != nil {
		return model.RemediationGitOpsArtifact{}, err
	} else if !ok {
		return model.RemediationGitOpsArtifact{}, ErrProposalNotFound
	}

	nowAt := s.now().UTC().Format(time.RFC3339)
	item := model.RemediationGitOpsArtifact{
		ProposalID:  needle,
		Artifact:    normalizeGitOpsArtifact(artifact),
		GeneratedBy: fallbackString(generatedBy, "unknown"),
		GeneratedAt: nowAt,
		UpdatedAt:   nowAt,
	}
	instructionsJSON, err := json.Marshal(item.Artifact.Instructions)
	if err != nil {
		return model.RemediationGitOpsArtifact{}, err
	}

	result, err := s.db.ExecContext(
		ctx,
		s.bind(`UPDATE remediation_gitops_artifacts
		    SET support_level = ?, strategy = ?, summary = ?, branch_name = ?, pr_title = ?,
		        commit_message = ?, target_path = ?, target_kind = ?, target_namespace = ?,
		        target_name = ?, format = ?, artifact_body = ?, instructions_json = ?,
		        generated_by = ?, generated_at = ?, updated_at = ?
		  WHERE proposal_id = ?`),
		string(item.Artifact.SupportLevel),
		item.Artifact.Strategy,
		item.Artifact.Summary,
		item.Artifact.BranchName,
		item.Artifact.PRTitle,
		item.Artifact.CommitMessage,
		item.Artifact.TargetPath,
		item.Artifact.TargetKind,
		item.Artifact.TargetNamespace,
		item.Artifact.TargetName,
		item.Artifact.Format,
		item.Artifact.ArtifactBody,
		string(instructionsJSON),
		item.GeneratedBy,
		item.GeneratedAt,
		item.UpdatedAt,
		item.ProposalID,
	)
	if err != nil {
		return model.RemediationGitOpsArtifact{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return model.RemediationGitOpsArtifact{}, err
	}
	if affected == 0 {
		_, err = s.db.ExecContext(
			ctx,
			s.bind(`INSERT INTO remediation_gitops_artifacts (
				proposal_id, support_level, strategy, summary, branch_name, pr_title, commit_message,
				target_path, target_kind, target_namespace, target_name, format, artifact_body,
				instructions_json, generated_by, generated_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
			item.ProposalID,
			string(item.Artifact.SupportLevel),
			item.Artifact.Strategy,
			item.Artifact.Summary,
			item.Artifact.BranchName,
			item.Artifact.PRTitle,
			item.Artifact.CommitMessage,
			item.Artifact.TargetPath,
			item.Artifact.TargetKind,
			item.Artifact.TargetNamespace,
			item.Artifact.TargetName,
			item.Artifact.Format,
			item.Artifact.ArtifactBody,
			string(instructionsJSON),
			item.GeneratedBy,
			item.GeneratedAt,
			item.UpdatedAt,
		)
	}
	if err != nil {
		return model.RemediationGitOpsArtifact{}, err
	}
	return cloneGitOpsArtifact(item), nil
}

func normalizeGitOpsArtifact(in model.GitOpsArtifact) model.GitOpsArtifact {
	out := in
	out.SupportLevel = model.GitOpsSupportLevel(strings.TrimSpace(string(in.SupportLevel)))
	out.Strategy = strings.TrimSpace(in.Strategy)
	out.Summary = strings.TrimSpace(in.Summary)
	out.BranchName = strings.TrimSpace(in.BranchName)
	out.PRTitle = strings.TrimSpace(in.PRTitle)
	out.CommitMessage = strings.TrimSpace(in.CommitMessage)
	out.TargetPath = strings.TrimSpace(in.TargetPath)
	out.TargetKind = strings.TrimSpace(in.TargetKind)
	out.TargetNamespace = strings.TrimSpace(in.TargetNamespace)
	out.TargetName = strings.TrimSpace(in.TargetName)
	out.Format = strings.TrimSpace(in.Format)
	out.ArtifactBody = strings.TrimSpace(in.ArtifactBody)
	out.Instructions = append([]string(nil), in.Instructions...)
	return out
}

func cloneGitOpsArtifact(in model.RemediationGitOpsArtifact) model.RemediationGitOpsArtifact {
	out := in
	out.Artifact.Instructions = append([]string(nil), in.Artifact.Instructions...)
	return out
}

type gitOpsArtifactScanner interface {
	Scan(dest ...any) error
}

func scanGitOpsArtifact(scanner gitOpsArtifactScanner) (model.RemediationGitOpsArtifact, error) {
	var (
		item             model.RemediationGitOpsArtifact
		supportLevel     string
		instructionsJSON string
	)
	if err := scanner.Scan(
		&item.ProposalID,
		&supportLevel,
		&item.Artifact.Strategy,
		&item.Artifact.Summary,
		&item.Artifact.BranchName,
		&item.Artifact.PRTitle,
		&item.Artifact.CommitMessage,
		&item.Artifact.TargetPath,
		&item.Artifact.TargetKind,
		&item.Artifact.TargetNamespace,
		&item.Artifact.TargetName,
		&item.Artifact.Format,
		&item.Artifact.ArtifactBody,
		&instructionsJSON,
		&item.GeneratedBy,
		&item.GeneratedAt,
		&item.UpdatedAt,
	); err != nil {
		return model.RemediationGitOpsArtifact{}, err
	}
	item.Artifact.SupportLevel = model.GitOpsSupportLevel(supportLevel)
	if strings.TrimSpace(instructionsJSON) != "" {
		if err := json.Unmarshal([]byte(instructionsJSON), &item.Artifact.Instructions); err != nil {
			return model.RemediationGitOpsArtifact{}, err
		}
	}
	return cloneGitOpsArtifact(item), nil
}
