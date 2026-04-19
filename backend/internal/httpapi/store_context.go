package httpapi

import (
	"context"

	"kubelens-backend/internal/model"
)

type incidentContextStore interface {
	CreateContext(ctx context.Context, incident model.Incident) (model.Incident, error)
	ListContext(ctx context.Context) ([]model.Incident, error)
	GetContext(ctx context.Context, id string) (model.Incident, bool, error)
	PatchStepStatusContext(ctx context.Context, id string, stepID string, target model.RunbookStepStatus) (model.Incident, error)
	ResolveContext(ctx context.Context, id string) (model.Incident, error)
	AssociateRemediationContext(ctx context.Context, incidentID string, proposalID string) error
}

type remediationContextStore interface {
	SaveProposalsContext(ctx context.Context, proposals []model.RemediationProposal) ([]model.RemediationProposal, error)
	ListContext(ctx context.Context) ([]model.RemediationProposal, error)
	GetContext(ctx context.Context, id string) (model.RemediationProposal, bool, error)
	ApproveContext(ctx context.Context, id string, user string) (model.RemediationProposal, error)
	RejectContext(ctx context.Context, id string, user string, reason string) (model.RemediationProposal, error)
	MarkExecutedContext(ctx context.Context, id string, user string, result string) (model.RemediationProposal, error)
}

type postmortemContextStore interface {
	CreateContext(ctx context.Context, postmortem model.Postmortem) (model.Postmortem, error)
	ListContext(ctx context.Context) ([]model.Postmortem, error)
	GetContext(ctx context.Context, id string) (model.Postmortem, bool, error)
	GetByIncidentIDContext(ctx context.Context, incidentID string) (model.Postmortem, bool, error)
}

func createIncidentWithContext(ctx context.Context, store incidentStore, incident model.Incident) (model.Incident, error) {
	if contextual, ok := store.(incidentContextStore); ok {
		return contextual.CreateContext(ctx, incident)
	}
	return store.Create(incident), nil
}

func listIncidentsWithContext(ctx context.Context, store incidentStore) ([]model.Incident, error) {
	if contextual, ok := store.(incidentContextStore); ok {
		return contextual.ListContext(ctx)
	}
	return store.List(), nil
}

func getIncidentWithContext(ctx context.Context, store incidentStore, id string) (model.Incident, bool, error) {
	if contextual, ok := store.(incidentContextStore); ok {
		return contextual.GetContext(ctx, id)
	}
	item, ok := store.Get(id)
	return item, ok, nil
}

func patchIncidentStepWithContext(
	ctx context.Context,
	store incidentStore,
	id string,
	stepID string,
	target model.RunbookStepStatus,
) (model.Incident, error) {
	if contextual, ok := store.(incidentContextStore); ok {
		return contextual.PatchStepStatusContext(ctx, id, stepID, target)
	}
	return store.PatchStepStatus(id, stepID, target)
}

func resolveIncidentWithContext(ctx context.Context, store incidentStore, id string) (model.Incident, error) {
	if contextual, ok := store.(incidentContextStore); ok {
		return contextual.ResolveContext(ctx, id)
	}
	return store.Resolve(id)
}

func associateIncidentRemediationWithContext(
	ctx context.Context,
	store incidentStore,
	incidentID string,
	proposalID string,
) error {
	if contextual, ok := store.(incidentContextStore); ok {
		return contextual.AssociateRemediationContext(ctx, incidentID, proposalID)
	}
	return store.AssociateRemediation(incidentID, proposalID)
}

func saveRemediationsWithContext(
	ctx context.Context,
	store remediationStore,
	proposals []model.RemediationProposal,
) ([]model.RemediationProposal, error) {
	if contextual, ok := store.(remediationContextStore); ok {
		return contextual.SaveProposalsContext(ctx, proposals)
	}
	return store.SaveProposals(proposals), nil
}

func listRemediationsWithContext(ctx context.Context, store remediationStore) ([]model.RemediationProposal, error) {
	if contextual, ok := store.(remediationContextStore); ok {
		return contextual.ListContext(ctx)
	}
	return store.List(), nil
}

func getRemediationWithContext(
	ctx context.Context,
	store remediationStore,
	id string,
) (model.RemediationProposal, bool, error) {
	if contextual, ok := store.(remediationContextStore); ok {
		return contextual.GetContext(ctx, id)
	}
	item, ok := store.Get(id)
	return item, ok, nil
}

func approveRemediationWithContext(
	ctx context.Context,
	store remediationStore,
	id string,
	user string,
) (model.RemediationProposal, error) {
	if contextual, ok := store.(remediationContextStore); ok {
		return contextual.ApproveContext(ctx, id, user)
	}
	return store.Approve(id, user)
}

func rejectRemediationWithContext(
	ctx context.Context,
	store remediationStore,
	id string,
	user string,
	reason string,
) (model.RemediationProposal, error) {
	if contextual, ok := store.(remediationContextStore); ok {
		return contextual.RejectContext(ctx, id, user, reason)
	}
	return store.Reject(id, user, reason)
}

func executeRemediationWithContext(
	ctx context.Context,
	store remediationStore,
	id string,
	user string,
	result string,
) (model.RemediationProposal, error) {
	if contextual, ok := store.(remediationContextStore); ok {
		return contextual.MarkExecutedContext(ctx, id, user, result)
	}
	return store.MarkExecuted(id, user, result)
}

func createPostmortemWithContext(
	ctx context.Context,
	store postmortemStore,
	postmortem model.Postmortem,
) (model.Postmortem, error) {
	if contextual, ok := store.(postmortemContextStore); ok {
		return contextual.CreateContext(ctx, postmortem)
	}
	return store.Create(postmortem)
}

func listPostmortemsWithContext(ctx context.Context, store postmortemStore) ([]model.Postmortem, error) {
	if contextual, ok := store.(postmortemContextStore); ok {
		return contextual.ListContext(ctx)
	}
	return store.List(), nil
}

func getPostmortemWithContext(ctx context.Context, store postmortemStore, id string) (model.Postmortem, bool, error) {
	if contextual, ok := store.(postmortemContextStore); ok {
		return contextual.GetContext(ctx, id)
	}
	item, ok := store.Get(id)
	return item, ok, nil
}

func getPostmortemByIncidentWithContext(
	ctx context.Context,
	store postmortemStore,
	incidentID string,
) (model.Postmortem, bool, error) {
	if contextual, ok := store.(postmortemContextStore); ok {
		return contextual.GetByIncidentIDContext(ctx, incidentID)
	}
	item, ok := store.GetByIncidentID(incidentID)
	return item, ok, nil
}
