package remediation

import (
	"fmt"
	"strings"
	"time"

	"kubelens-backend/internal/gitops"
	"kubelens-backend/internal/model"
)

func BuildGitOpsArtifact(
	proposal model.RemediationProposal,
	inventory gitops.WorkloadInventory,
	now time.Time,
) model.GitOpsArtifact {
	switch proposal.Kind {
	case model.RemediationKindRestartPod:
		return buildRestartArtifact(proposal, inventory, now)
	case model.RemediationKindRollbackDeployment:
		return buildRollbackArtifact(proposal)
	case model.RemediationKindCordonNode:
		return buildCordonArtifact(proposal)
	default:
		return gitops.BuildAdvisoryArtifact(
			"Proposal kind is not currently modeled as a declarative GitOps patch.",
			"manual_review",
			gitops.WorkloadTarget{Namespace: proposal.Namespace, Name: proposal.Resource},
			fmt.Sprintf("remediation/%s-review", gitops.Slug(proposal.ID)),
			fmt.Sprintf("Review remediation %s", proposal.ID),
			fmt.Sprintf("docs(remediation): capture %s review notes", proposal.ID),
			fmt.Sprintf("# Manual Remediation Review\n\n- Proposal: %s\n- Reason: %s", proposal.ID, proposal.Reason),
			[]string{"Review the proposal manually and translate it into the appropriate infrastructure repository workflow."},
		)
	}
}

func buildRestartArtifact(
	proposal model.RemediationProposal,
	inventory gitops.WorkloadInventory,
	now time.Time,
) model.GitOpsArtifact {
	target, ok := gitops.ResolvePodWorkload(proposal.Namespace, proposal.Resource, inventory)
	branch := fmt.Sprintf("remediation/%s-rollout-restart", gitops.Slug(proposal.ID))
	prTitle := fmt.Sprintf("Restart workload for remediation %s", proposal.ID)
	commitMessage := fmt.Sprintf("chore(remediation): rollout restart %s", proposal.ID)
	restartAt := now.UTC().Format(time.RFC3339)
	if ok {
		body := strings.Join([]string{
			"apiVersion: apps/v1",
			fmt.Sprintf("kind: %s", gitopsKindTitle(target.Kind)),
			"metadata:",
			fmt.Sprintf("  name: %s", target.Name),
			fmt.Sprintf("  namespace: %s", target.Namespace),
			"spec:",
			"  template:",
			"    metadata:",
			"      annotations:",
			fmt.Sprintf("        kubectl.kubernetes.io/restartedAt: %q", restartAt),
			fmt.Sprintf("        kubelens.ai/remediation-id: %q", proposal.ID),
		}, "\n")
		return gitops.BuildPatchArtifact(
			fmt.Sprintf("Trigger a declarative rollout restart for %s %s/%s.", target.Kind, target.Namespace, target.Name),
			"rollout_restart",
			target,
			branch,
			prTitle,
			commitMessage,
			body,
			[]string{
				"Commit the annotation patch in the workload repository instead of restarting the pod directly in-cluster.",
				"Wait for the GitOps controller to reconcile and observe rollout health before closing the incident.",
			},
		)
	}

	target = gitops.WorkloadTarget{Namespace: proposal.Namespace, Name: proposal.Resource}
	body := strings.Join([]string{
		"# Rollout Restart Advisory",
		"",
		fmt.Sprintf("- Proposal: %s", proposal.ID),
		fmt.Sprintf("- Pod: %s/%s", strings.TrimSpace(proposal.Namespace), strings.TrimSpace(proposal.Resource)),
		fmt.Sprintf("- Reason: %s", strings.TrimSpace(proposal.Reason)),
		"",
		"The owning workload could not be resolved automatically. Translate this restart request into a Deployment, StatefulSet, or DaemonSet pod-template annotation patch in Git.",
	}, "\n")
	return gitops.BuildAdvisoryArtifact(
		"Owning workload could not be resolved automatically for rollout restart.",
		"rollout_restart_advisory",
		target,
		branch,
		prTitle,
		commitMessage,
		body,
		[]string{
			"Locate the pod owner first, then apply a pod-template annotation restart through the GitOps repository.",
		},
	)
}

func buildRollbackArtifact(proposal model.RemediationProposal) model.GitOpsArtifact {
	target := gitops.WorkloadTarget{
		Kind:      "deployment",
		Namespace: proposal.Namespace,
		Name:      proposal.Resource,
	}
	body := strings.Join([]string{
		"# Rollback Request",
		"",
		fmt.Sprintf("- Proposal: %s", proposal.ID),
		fmt.Sprintf("- Deployment: %s/%s", strings.TrimSpace(proposal.Namespace), strings.TrimSpace(proposal.Resource)),
		fmt.Sprintf("- Reason: %s", strings.TrimSpace(proposal.Reason)),
		"",
		"Rollback should be executed from Git history, Helm history, or the GitOps controller revision UI rather than by mutating the live cluster directly.",
		"",
		"Recommended pull request note:",
		fmt.Sprintf("> Revert %s/%s to the last known good revision and link this remediation proposal for audit context.", strings.TrimSpace(proposal.Namespace), strings.TrimSpace(proposal.Resource)),
	}, "\n")
	return gitops.BuildAdvisoryArtifact(
		"Capture the rollback request in Git and revert the workload to the last known good revision.",
		"rollback_request",
		target,
		fmt.Sprintf("remediation/%s-rollback", gitops.Slug(proposal.ID)),
		fmt.Sprintf("Request rollback for %s", proposal.ID),
		fmt.Sprintf("docs(remediation): capture rollback request %s", proposal.ID),
		body,
		[]string{
			"Find the last healthy revision in Git or the deployment system history.",
			"Open a revert-style pull request and reference the remediation proposal ID in the description.",
			"Let the GitOps controller apply the rollback and verify replica health before closing the proposal.",
		},
	)
}

func buildCordonArtifact(proposal model.RemediationProposal) model.GitOpsArtifact {
	target := gitops.WorkloadTarget{Name: proposal.Resource}
	body := strings.Join([]string{
		"# Node Maintenance Advisory",
		"",
		fmt.Sprintf("- Proposal: %s", proposal.ID),
		fmt.Sprintf("- Node: %s", strings.TrimSpace(proposal.Resource)),
		fmt.Sprintf("- Reason: %s", strings.TrimSpace(proposal.Reason)),
		"",
		"Node cordon is usually an operational action rather than a repository-managed manifest change. If the node is part of declarative infrastructure, record the maintenance intent in the infra repo or maintenance calendar instead of executing it ad hoc.",
	}, "\n")
	return gitops.BuildAdvisoryArtifact(
		"Record node maintenance intent in infrastructure automation or the operations runbook.",
		"node_maintenance_request",
		target,
		fmt.Sprintf("remediation/%s-node-maintenance", gitops.Slug(proposal.ID)),
		fmt.Sprintf("Record maintenance plan for %s", proposal.ID),
		fmt.Sprintf("docs(remediation): capture node maintenance request %s", proposal.ID),
		body,
		[]string{
			"Coordinate node maintenance through infrastructure automation or an approved change window.",
			"Attach the advisory note to the maintenance ticket so the action remains audited.",
		},
	)
}

func gitopsKindTitle(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "deployment":
		return "Deployment"
	case "statefulset":
		return "StatefulSet"
	case "daemonset":
		return "DaemonSet"
	default:
		return "Deployment"
	}
}
