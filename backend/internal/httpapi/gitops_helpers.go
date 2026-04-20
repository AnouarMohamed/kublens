package httpapi

import (
	"context"

	"kubelens-backend/internal/gitops"
	"kubelens-backend/internal/model"
)

func collectGitOpsWorkloadInventory(ctx context.Context, cluster ClusterReader) gitops.WorkloadInventory {
	return gitops.WorkloadInventory{
		Deployments:  listGitOpsResources(ctx, cluster, "deployments"),
		StatefulSets: listGitOpsResources(ctx, cluster, "statefulsets"),
		DaemonSets:   listGitOpsResources(ctx, cluster, "daemonsets"),
	}
}

func listGitOpsResources(ctx context.Context, cluster ClusterReader, kind string) []model.ResourceRecord {
	items, err := cluster.ListResources(ctx, kind)
	if err != nil {
		return nil
	}
	return items
}
