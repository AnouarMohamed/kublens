# KubLens-AI Refactoring Status

This document tracks the frontend hook and backend HTTP routing refactors. Keep it current whenever ownership boundaries or route composition change.

## Current Status

| Area                              | Status      | Notes                                                                                                                        |
| --------------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Nodes frontend hook decomposition | Complete    | `useNodesData.ts` is now a compatibility facade over smaller hooks.                                                          |
| Node view composition             | Complete    | `src/views/nodes/index.tsx` consumes the facade while action/list/bulk state lives in dedicated hooks.                       |
| Pod HTTP controller               | Complete    | Pod list/detail/log/event/action routes are owned by `PodController`.                                                        |
| Node HTTP controller              | Complete    | Node list/detail/scope/maintenance routes are owned by `NodeController`.                                                     |
| Resource HTTP controller          | Complete    | Generic resource list, YAML apply, scale, restart, and rollback routes are owned by `ResourceController`.                    |
| Remaining backend route domains   | In progress | Observability, ops, auth/system, incident, remediation, memory, and assistant routes still mount through `*Server` handlers. |

## Frontend Refactor

The original `src/views/nodes/hooks/useNodesData.ts` mixed list loading, search filtering, cordon/drain actions, bulk selection, rule evaluation, and alert lifecycle updates. That file now preserves the public hook shape for the view and tests while delegating responsibilities to focused hooks:

- `src/views/nodes/hooks/useNodeList.ts`
  - owns node/event loading, search state, filtering, alert lifecycle loading, and stream-triggered refreshes.
- `src/views/nodes/hooks/useNodeActions.ts`
  - owns detail modal retrieval and individual node actions: cordon, uncordon, drain preview, and drain.
- `src/views/nodes/hooks/useNodeBulkActions.ts`
  - owns selection state plus bulk cordon, uncordon, and drain flows.
- `src/views/nodes/hooks/useNodeAlertActions.ts`
  - owns alert dispatch and lifecycle mutation behavior.

Keep `useNodesData.ts` as the compatibility facade until the view and tests no longer need a single aggregate hook.

## Backend Refactor

`backend/internal/httpapi/server.go` now delegates API mounting through `routes_mount.go`. The next layer of refactoring is to keep domain routes behind controllers that receive only the dependencies they need.

Completed controller splits:

- `backend/internal/httpapi/handlers_cluster_pods.go`
  - `PodController` owns `/api/pods` and nested pod routes.
  - Injects `ClusterReader`, logger, JSON decoder, and prediction-cache invalidation callback.
- `backend/internal/httpapi/handlers_cluster_nodes.go`
  - `NodeController` owns `/api/nodes` and nested node routes.
  - Injects `ClusterReader`, audit log, clock, JSON decoder, and prediction-cache invalidation callback.
- `backend/internal/httpapi/handlers_cluster_resources.go`
  - `ResourceController` owns `/api/resources` and nested generic resource routes.
  - Injects `ClusterReader`, audit log, clock, JSON decoder, manifest risk evaluator, and prediction-cache invalidation callback.
  - Preserves Risk Guard force-override audit behavior.
- `backend/internal/httpapi/routes_mount.go`
  - mounts `PodController.Routes()` at `/pods`.
  - mounts `NodeController.Routes()` at `/nodes`.
  - mounts `ResourceController.Routes()` at `/resources`.

## Next Refactor Candidates

1. Extract observability controllers.
   - Candidate groups: metrics/prometheus/SLO/rightsizing, predictions, ghost simulations, stream endpoints, alert lifecycle.
2. Extract operations controllers.
   - Candidate groups: incidents/postmortems, remediation, memory, assistant/RAG, Risk Guard.
3. Extract auth/system controllers after route-level dependencies are clear.
   - Candidate groups: health/ready/version/runtime/OpenAPI, auth session/login/logout, cluster selection.
4. Revisit `Server` dependencies after each controller split.
   - Move dependencies out of `Server` only when no remaining server-owned route or middleware needs them.

## Quality Gates

Run the relevant gates for every refactor step:

```bash
npm run lint
npm run test:web
npm run test:go
```

For backend-only controller movement, at minimum run:

```bash
cd backend && go test ./internal/httpapi
```
