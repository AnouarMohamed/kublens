# Backend Internal Modules

`backend/internal` is organized by domain and responsibility:

- `ai/` -> assistant provider interfaces + implementations
- `apperrors/` -> shared sentinel/domain errors
- `auth/` -> JWT/OIDC auth, roles, and request principal handling
- `cluster/` -> Kubernetes data access, mapping, and actions
- `config/` -> env parsing, mode defaults, and startup validation
- `bootstrap/` -> dependency assembly and server construction
- `diagnostics/` -> health scoring + issue inference
- `events/` -> in-process event bus for streaming updates
- `alerts/` -> alert dispatch and webhook channel integrations
- `httpapi/` -> HTTP handlers, routing, transport concerns
- `intelligence/` -> deterministic diagnostic engine and scoring
- `model/` -> canonical backend API models
- `rag/` -> documentation retrieval and grounding for assistant responses
- `incident/` -> incident lifecycle and runbook progression
- `remediation/` -> remediation proposal + execution
- `memory/` -> runbook/fix persistence and retrieval
- `postmortem/` -> postmortem generation and storage
- `chatops/` -> Slack notification formatting and delivery
- `observability/` -> OTEL tracing bootstrap
- `riskguard/` -> manifest risk analysis
- `state/` -> informer-backed cluster cache

Navigation tips:

- Start at `cmd/server/main.go` for runtime wiring.
- Follow request flow in `internal/httpapi/server.go`.
- Core Kubernetes interactions are in `internal/cluster/`.

Use-case file conventions:

- `cluster/`
  - `query_*` -> read/list/detail operations
  - `command_*` -> mutating actions
  - `mapper_*` -> model mapping logic
  - `service_*` -> service lifecycle/cache/runtime wiring
  - `support_*` -> shared utility helpers
- `httpapi/`
  - `routes_mount.go` -> API route composition by domain
  - `handlers_*` -> endpoint-group handlers; domain controllers expose `Routes()` when a route group has been decoupled from `Server`
  - `assistant_*` -> assistant orchestration helpers
  - `auth_*`/`audit*` -> auth, RBAC and audit transport concerns
- `diagnostics/`
  - `analysis_*` -> diagnosis and scoring engine logic
  - `present_*` -> narrative/output formatting
