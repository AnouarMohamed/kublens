# KubLens-AI Architecture and Refactoring Guidance

This document is the authoritative architecture guide for agents working on this repository. It defines the target system shape, the migration strategy, and the non-negotiable engineering rules for changes that affect structure, boundaries, or runtime ownership.

## Mission

KubeLens AI is an evidence-first Kubernetes operations platform. The product goal is to help operators detect risk, understand why it exists, simulate likely impact, and execute governed remediation with auditability. The architecture must optimize for correctness, safety, observability, and gradual evolution rather than for premature abstraction.

## Architectural Principles

1. Build around domains, not around incidental folders.
   - The core domains are cluster state, incidents, remediation, diagnostics, assistant/RAG, memory, alerting, predictions, and Ghost simulation.
   - Each domain should own its own data model, business rules, and interfaces.

2. Prefer a modular monolith first.
   - Do not rewrite the system into microservices as an initial move.
   - Keep the system cohesive while enforcing strong module boundaries, clear interfaces, and explicit ownership.

3. Separate runtime concerns cleanly.
   - UI, API transport, domain logic, persistence, streaming, and intelligence must remain conceptually distinct.
   - Avoid mixing these concerns in the same layer.

4. Make safety and governance first-class.
   - Authentication, RBAC, write gates, auditability, and approval workflows must remain intact across any refactor.
   - Refactors must not weaken security posture or operational control.

5. Keep deterministic logic independent from optional AI/ML.
   - Deterministic diagnostics, evidence generation, and fallback behavior are the product foundation.
   - AI-assisted features should augment, not replace, the reliable core.

6. Design for evolution, not for purity.
   - The architecture should be easy to split into separate services later if real scaling or ownership needs emerge.
   - Build with interfaces and contracts that make extraction feasible without rewrites.

7. Prefer explicit contracts over hidden coupling.
   - Cross-module communication should go through typed interfaces, APIs, or events.
   - Avoid direct shared-state access or implicit dependency chains.

8. Optimize for operability.
   - Every significant architectural change should preserve observability, debugging capability, and rollback friendliness.

## Target Architecture

### 1. Frontend

- React + TypeScript + Vite
- Feature-oriented composition
- Thin view components and hooks
- API calls through typed clients and shared contracts
- State should be organized by feature domain rather than by page-only concerns

### 2. Core backend API

- A single Go API runtime remains the primary entry point for most product workflows
- Responsible for routing, auth, middleware, request validation, audit, streaming, and shared runtime wiring
- Domain-specific logic should live in focused internal packages rather than in monolithic handlers

### 3. Domain modules

The backend should be organized around the following logical domains:

- Cluster state and inventory
- Diagnostics and analysis
- Incidents and postmortems
- Remediation and GitOps workflows
- Memory and runbooks
- Assistant and RAG orchestration
- Alerts and chatops
- Rightsizing and SLO analysis
- Ghost simulation and change-risk modeling
- Prediction and model governance

Each domain should own:

- its business rules
- its persistence boundaries
- its API surface
- its error handling
- its tests

### 4. Predictor and intelligence runtime

- The predictor is already a distinct runtime concern and should remain isolated from the core API workflow
- It should expose a stable contract for inference and model-health reporting
- The core backend should use it as an external dependency with graceful fallback behavior

### 5. Optional heavy simulation runtime

- Ghost simulation is a specialized domain and should remain isolated from the main operational path unless it is directly required
- It should be deployable as an independent runtime if its complexity grows

### 6. Data architecture

- PostgreSQL for transactional and workflow state
- Time-series storage for metrics and telemetry where appropriate
- Redis for caching or queueing where useful
- Object storage for artifacts and exports when needed
- Avoid sharing a single database across domains in a way that creates hidden coupling

### 7. Eventing and async integration

- Use synchronous request/response for direct user-facing workflows
- Use events or queues for asynchronous workflows such as incident creation, remediation approval, alert propagation, or prediction refresh
- The event model should be explicit and documented

### 8. Platform and operations

- Kubernetes and Helm for deployment
- OpenTelemetry, Prometheus, and Grafana for observability
- CI/CD with automated tests, feature flags, and progressive rollout

## Repository-Specific Boundaries

The repository already contains the right architectural skeleton. Preserve and strengthen it.

### Frontend

- [src](src) contains the UI shell, views, feature modules, hooks, and typed API usage
- Keep view components thin and feature logic in hooks or feature modules
- Avoid embedding business rules or backend assumptions directly into UI components

### Backend runtime

- [backend/internal/httpapi](backend/internal/httpapi) owns transport concerns and route composition
- [backend/internal](backend/internal) contains the domain packages
- Each domain package should expose clear interfaces and avoid reaching into unrelated packages unnecessarily

### Predictor

- [predictor](predictor) is a separate runtime concern with its own packaging and tests
- Keep it independent from the main API’s core execution path where possible

### Ghost engine

- [ghost-engine](ghost-engine) is a specialized simulation runtime and should not be treated as ordinary business logic inside the core API

## Service Boundary Decision Rules

Keep a capability inside the current modular backend when all of the following are true:

- it shares the same deployment and scaling profile
- it does not require independent failure isolation
- it does not need separate ownership or release cadence
- it does not create unacceptable coupling with other modules

Extract it into a separate service only when one or more of these are true:

- it has a distinct scalability profile
- it has different operational requirements or failure modes
- it needs independent deployment or ownership
- it becomes a bottleneck or hot path that should not affect the main user workflow
- it needs stronger isolation due to safety, performance, or compliance needs

For this repository, the predictor and Ghost simulation are the most obvious candidates for future service extraction. The rest of the product should stay modular and cohesive until there is real evidence that splitting is necessary.

## Migration Strategy

### Phase 0: Stabilize contracts

- Preserve APIs and shared contracts
- Document boundaries before changing ownership
- Identify the domain boundary of each refactor before implementation

### Phase 1: Strengthen domain ownership

- Move logic out of generic handlers into domain-owned packages
- Keep route composition thin and transport-focused
- Make dependencies explicit and narrow

### Phase 2: Isolate intelligence and simulation workloads

- Keep predictor integration behind a stable interface
- Preserve deterministic fallback behavior
- Keep model-health telemetry and governance visible

### Phase 3: Introduce async integration where it is genuinely useful

- Add events or queues only where asynchronous operation improves resilience or workflow clarity
- Do not introduce event-driven complexity just to appear modern

### Phase 4: Extract services selectively

- Split only when operational pain or scaling needs justify it
- Prefer one service at a time, with a clear ownership boundary and migration path

## Engineering Rules for Coding Agents

When working on this repository, follow these rules without exception:

1. Read the architecture context first
   - Read [README.md](README.md), [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/FEATURES.md](docs/FEATURES.md), and the relevant package directories before making structural changes.

2. Preserve product safety
   - Authentication, RBAC, write gating, and audit behavior must remain intact.

3. Keep transport code separate from business logic
   - HTTP handlers and controllers should orchestrate, not embed complex rules.

4. Preserve backward compatibility where possible
   - Avoid breaking API surfaces unless the change is explicitly approved as a contract change.

5. Prefer explicit interfaces over hidden shared state
   - If two domains need to collaborate, define a clear contract.

6. Keep the predictor and core platform decoupled
   - Do not let the main API path become dependent on prediction internals in a way that makes the system brittle.

7. Do not create premature services
   - If something can be a module, keep it a module until there is evidence that it should become a service.

8. Avoid architecture churn for its own sake
   - Every refactor should solve a concrete problem: clarity, scalability, safety, testability, or maintainability.

9. Document new boundaries
   - If a refactor changes ownership, update the architecture docs and route ownership notes.

10. Verify behavior after structural changes

- Use the relevant tests, linters, and runtime checks before considering the work complete.

## Forbidden Patterns

- Rewriting the system into microservices before the product has earned that complexity
- Moving logic into generic handlers that no longer have clear ownership
- Creating hidden module dependencies through direct package imports that bypass interfaces
- Allowing the frontend to contain core business logic that belongs in backend domains
- Coupling the core API directly to predictor internals without fallback and contracts
- Introducing shared database access patterns that make domain boundaries meaningless
- Adding asynchronous infrastructure without clear operational semantics or observability

## Definition of Done for Architectural Changes

A structural change is complete only when all of the following are true:

- the responsibility boundary is clear
- the contract is explicit and documented
- tests and validations still pass
- observability and safety remain intact
- the change can be reasoned about without hidden coupling
- the system remains easy to evolve toward a more service-oriented model later if needed

## Short Version

The correct target for this repository is not “microservices everywhere.” The correct target is a well-structured, domain-oriented platform with a modular monolith foundation, explicit boundaries, strong governance, and the ability to extract services later when the product truly demands it.

This file should be treated as the default architectural reference for any agent making structural changes.

---

## Refactoring Status

This document tracks the frontend hook and backend HTTP routing refactors. Keep it current whenever ownership boundaries or route composition change.

## Current Status

| Area                              | Status      | Notes                                                                                                                          |
| --------------------------------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------ |
| Nodes frontend hook decomposition | Complete    | `useNodesData.ts` is now a compatibility facade over smaller hooks.                                                            |
| Node view composition             | Complete    | `src/views/nodes/index.tsx` consumes the facade while action/list/bulk state lives in dedicated hooks.                         |
| Pod HTTP controller               | Complete    | Pod list/detail/log/event/action routes are owned by `PodController`.                                                          |
| Node HTTP controller              | Complete    | Node list/detail/scope/maintenance routes are owned by `NodeController`.                                                       |
| Resource HTTP controller          | Complete    | Generic resource list, YAML apply, scale, restart, and rollback routes are owned by `ResourceController`.                      |
| Metrics HTTP controller           | Complete    | JSON and Prometheus metrics routes are owned by `MetricsController`.                                                           |
| SLO HTTP controller               | Complete    | SLO overview route is owned by `SLOController`.                                                                                |
| Rightsizing HTTP controller       | Complete    | Rightsizing overview route is owned by `RightsizingController`.                                                                |
| Audit HTTP controller             | Complete    | Audit read route is owned by `AuditController`; audit middleware remains server-owned.                                         |
| Stream HTTP controller            | Complete    | SSE and WebSocket stream routes are owned by `StreamController`.                                                               |
| Prediction HTTP controller        | Complete    | Prediction and predictive-incident alias routes are owned by `PredictionController`.                                           |
| Ghost HTTP controller             | Complete    | Ghost topology and simulation routes are owned by `GhostController`.                                                           |
| Alert HTTP controller             | Complete    | Alert dispatch, test, and lifecycle routes are owned by `AlertController`.                                                     |
| Remaining backend route domains   | In progress | Ops, auth/system, incident, remediation, memory, assistant, RAG, and Risk Guard routes still mount through `*Server` handlers. |

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
- `backend/internal/httpapi/metrics_prometheus.go`
  - `MetricsController` owns `/api/metrics` and `/api/metrics/prometheus`.
  - Injects request metrics, docs retriever, and runtime snapshot callback.
- `backend/internal/httpapi/handlers_slo.go`
  - `SLOController` owns `/api/slo`.
  - Injects request metrics, incident store, cluster stats callback, and clock.
- `backend/internal/httpapi/handlers_rightsizing.go`
  - `RightsizingController` owns `/api/rightsizing`.
  - Injects `ClusterReader` and clock.
- `backend/internal/httpapi/audit.go`
  - `AuditController` owns `/api/audit`.
  - Audit-write middleware remains server-owned because it wraps all API routes.
- `backend/internal/httpapi/stream.go`
  - `StreamController` owns `/api/stream` and `/api/stream/ws`.
  - Injects `ClusterReader`, event bus, clock, cluster stats callback, and trusted CSRF domains.
- `backend/internal/httpapi/handlers_predictions.go`
  - `PredictionController` owns `/api/predictions` and `/api/predictive-incidents`.
  - Injects `ClusterReader`, predictor provider, logger, clock, prediction-cache callbacks, and predictor-health callbacks.
- `backend/internal/httpapi/handlers_ghost.go`
  - `GhostController` owns `/api/ghost/topology` and `/api/ghost/simulations`.
  - Injects `ClusterReader`, optional gRPC ghost client, remediation store, logger, clock, and JSON decoder.
- `backend/internal/httpapi/handlers_alerts.go`
  - `AlertController` owns `/api/alerts/dispatch`, `/api/alerts/test`, and `/api/alerts/lifecycle`.
  - Injects alert dispatcher, alert lifecycle store, and JSON decoder.
- `backend/internal/httpapi/routes_mount.go`
  - mounts `PodController.Routes()` at `/pods`.
  - mounts `NodeController.Routes()` at `/nodes`.
  - mounts `ResourceController.Routes()` at `/resources`.
  - mounts `MetricsController.Routes()` at `/metrics`.
  - mounts `SLOController.Routes()` at `/slo`.
  - mounts `RightsizingController.Routes()` at `/rightsizing`.
  - mounts remaining observability controllers for audit, stream, ghost, predictions, and alerts.

## Next Refactor Candidates

1. Extract operations controllers.
   - Candidate groups: incidents/postmortems, remediation, memory, assistant/RAG, Risk Guard.
2. Extract auth/system controllers after route-level dependencies are clear.
   - Candidate groups: health/ready/version/runtime/OpenAPI, auth session/login/logout, cluster selection.
3. Revisit `Server` dependencies after each controller split.
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
