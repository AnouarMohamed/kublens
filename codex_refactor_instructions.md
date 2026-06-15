# Codex Refactoring Instructions: KubLens-AI

This document provides a structured guide and technical specifications for Codex to refactor the frontend hooks and backend routing architecture in the **KubLens-AI** repository.

Follow these instructions to improve modularity, maintainability, and clean separation of concerns.

---

## 🚀 Part 1: Frontend Refactoring (Custom Hooks Decomposition)

### Target File

- [useNodesData.ts](file:///home/anouar/KubLens-AI/src/views/nodes/hooks/useNodesData.ts) (approx. 488 lines)

### The Problem

The hook contains too many responsibilities: fetching lists, searching/filtering, managing cordon/drain logic, managing bulk actions, evaluation rules, and alert lifecycle dispatches.

### Codex Task

Decompose [useNodesData.ts](file:///home/anouar/KubLens-AI/src/views/nodes/hooks/useNodesData.ts) into the following three smaller, single-responsibility hooks:

1. **`useNodeList.ts`**:
   - Manages basic loading states (`nodes`, `clusterEvents`).
   - Handles search filtering logic (`filteredNodes`).
   - Hooks up event stream updates via `useStreamRefresh`.
2. **`useNodeActions.ts`**:
   - Handles individual node actions: `cordon`, `uncordon`, `previewDrain`, `drain`, and detail modal retrieval (`openDetail`).
   - Updates target statuses in-place or triggers a lightweight list refresh callback.
3. **`useNodeBulkActions.ts`**:
   - Manages list selections (`selectedNodeNames`) and bulk selections.
   - Integrates actions for `bulkCordon`, `bulkUncordon`, and `bulkDrain`.

### Clean-Up Requirements

- Update the main view component `src/views/nodes/index.tsx` to pull state from these decomposed hooks.
- Verify that the unit tests in [useNodesData.test.tsx](file:///home/anouar/KubLens-AI/src/views/nodes/hooks/useNodesData.test.tsx) continue to pass.

---

## 🛠️ Part 2: Backend Refactoring (Routing & Handlers Decomposition)

### Target File

- [server.go](file:///home/anouar/KubLens-AI/backend/internal/httpapi/server.go)

### The Problem

The `Server` struct is a "God Object" holding every service, database handle, logger, and configuration. Its `Router` method (lines 365–475) registers over 50 endpoints directly, causing high coupling between transport logic and domain services.

### Codex Task

Decouple endpoint routing and handler functions by extracting domain-specific endpoints into separate mounting routers.

1. **Create Sub-Routers / Mounting Routers**:
   - Rather than registering everything on the main `Server` router, create helper controller structs or mount routes domain-by-domain.
   - Example structure for clusters/nodes/pods in `backend/internal/httpapi`:

     ```go
     // backend/internal/httpapi/pods.go
     type PodController struct {
         cluster ClusterReader
         logger  *slog.Logger
     }

     func NewPodController(c ClusterReader, l *slog.Logger) *PodController {
         return &PodController{cluster: c, logger: l}
     }

     func (pc *PodController) Routes() chi.Router {
         r := chi.NewRouter()
         r.Get("/", pc.handleListPods)
         r.Post("/", pc.handleCreatePod)
         r.Get("/{namespace}/{name}", pc.handlePodDetail)
         r.Delete("/{namespace}/{name}", pc.handleDeletePod)
         // ...
         return r
     }
     ```

2. **Mount Domains in main Router**:
   - In [server.go](file:///home/anouar/KubLens-AI/backend/internal/httpapi/server.go), mount the sub-routers:
     ```go
     r.Route(apiMountPrefix, func(api chi.Router) {
         api.Mount("/pods", NewPodController(s.cluster, s.logger).Routes())
         api.Mount("/nodes", NewNodeController(s.cluster, s.logger).Routes())
         // ...
     })
     ```
3. **Decouple Handlers**:
   - Move handler functions (e.g. `handlePods` from `handlers_cluster_pods.go`) into their corresponding controller structs.
   - Only pass the required interface dependency (e.g., `ClusterReader`, `incidentStore`) into the controller rather than passing the whole `Server` object.

---

## 🧪 Part 3: Validation & Quality Gate

After executing the refactors, make sure to execute all verification commands to ensure no regressions were introduced.

### Commands to Run

- **Type-check & Lint**:
  ```bash
  npm run lint
  ```
- **Frontend Tests**:
  ```bash
  npm run test:web
  ```
- **Backend Tests**:
  ```bash
  npm run test:go
  ```
