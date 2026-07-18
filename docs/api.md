# API Guide

KubeLens API is served under `/api`.  
Formal schema source of truth: `backend/internal/httpapi/openapi.yaml`.

This guide focuses on practical endpoint groups and operational behavior.

## Base URL

- Local: `http://localhost:3000/api`
- In-cluster: your service/ingress URL + `/api`

## Auth, RBAC, and write gating

### Login/session

1. `POST /auth/login` with `{ "token": "<token>" }`
2. Server validates token and sets an HttpOnly session cookie.
3. `GET /auth/session` returns current session state.
4. `POST /auth/logout` clears the session.

### Roles

- `viewer`: read + assist + stream
- `operator`: viewer + write routes (if write gate enabled)
- `admin`: operator + admin-level routes

### Write gate

Mutating cluster routes are additionally blocked unless `WRITE_ACTIONS_ENABLED=true`.

### Error shape

```json
{ "error": "message" }
```

## Endpoint groups

## System and observability

- `GET /healthz`
- `GET /readyz`
- `GET /readiness/enterprise`
- `GET /openapi.yaml`
- `GET /version`
- `GET /runtime`
- `GET /metrics`
- `GET /metrics/prometheus`
- `GET /slo`
- `GET /rightsizing`

Enterprise readiness checks add production posture signals for auth, write gating, durable storage, audit availability, predictor state, and cluster reachability. A failed enterprise check returns `503` with the same `HealthStatus` shape as `/readyz`.

## Auth and cluster context

- `GET /auth/session`
- `POST /auth/login`
- `POST /auth/logout`
- `GET /clusters`
- `POST /clusters/select`

## Streams and audit

- `GET /stream` (SSE)
- `GET /stream/ws` (WebSocket)
- `GET /audit`
- `GET /audit/{id}/verify`

Notes:

- WebSocket upgrades enforce same-origin/trusted-origin checks.
- Cross-origin upgrade attempts are rejected with `403`.
- Audit entries include `previousHash` and `hash` for tamper-evident verification of newly recorded events.

## Alerts

- `POST /alerts/dispatch`
- `POST /alerts/test`
- `GET /alerts/lifecycle`
- `POST /alerts/lifecycle`

## Cluster inventory and detail

- `GET /cluster-info`
- `GET /namespaces`
- `GET /pods`
- `GET /pods/{namespace}/{name}`
- `GET /pods/{namespace}/{name}/events`
- `GET /pods/{namespace}/{name}/logs`
- `GET /pods/{namespace}/{name}/logs/stream`
- `GET /pods/{namespace}/{name}/describe`
- `GET /nodes`
- `GET /nodes/{name}`
- `GET /nodes/{name}/pods`
- `GET /nodes/{name}/events`
- `GET /events`
- `GET /resources/{kind}`
- `GET /resources/{kind}/{namespace}/{name}/yaml`
- `GET /stats`

## Mutating cluster operations

- `POST /pods`
- `POST /pods/{namespace}/{name}/restart`
- `DELETE /pods/{namespace}/{name}`
- `POST /nodes/{name}/cordon`
- `POST /nodes/{name}/uncordon`
- `GET /nodes/{name}/drain/preview`
- `POST /nodes/{name}/drain`
- `PUT /resources/{kind}/{namespace}/{name}/yaml`
- `POST /resources/{kind}/{namespace}/{name}/scale`
- `POST /resources/{kind}/{namespace}/{name}/restart`
- `POST /resources/{kind}/{namespace}/{name}/rollback`

## Intelligence and assistant

- `GET /diagnostics`
- `GET /predictions`
- `GET /predictive-incidents` (backward-compatible alias)
- `GET /ghost/topology`
- `GET /ghost/simulations`
- `POST /ghost/simulations`
- `GET /ghost/simulations/{id}`
- `POST /assistant`
- `POST /assistant/references/feedback`
- `GET /rag/telemetry`

Ghost simulation responses include engine, topology hash, confidence, and limitations so operators can distinguish narrow node-drain previews from full digital-twin claims.

## Incident, remediation, memory, postmortem

- `POST /incidents`
- `GET /incidents`
- `GET /incidents/{id}`
- `GET /incidents/{id}/replay`
- `GET /incidents/{id}/evidence`
- `PATCH /incidents/{id}/steps/{step}`
- `POST /incidents/{id}/resolve`
- `POST /incidents/{id}/postmortem`
- `GET /postmortems`
- `GET /postmortems/{id}`
- `POST /remediation/propose`
- `GET /remediation`
- `GET /remediation/{id}/gitops`
- `POST /remediation/{id}/gitops`
- `POST /remediation/{id}/approve`
- `POST /remediation/{id}/execute`
- `POST /remediation/{id}/reject`
- `GET /memory/runbooks`
- `POST /memory/runbooks`
- `PUT /memory/runbooks/{id}`
- `GET /memory/fixes`
- `POST /memory/fixes`
- `POST /risk-guard/analyze`

## Example requests

### Login

```bash
curl -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"token":"viewer-token"}'
```

### Request diagnostics

```bash
curl -s http://localhost:3000/api/diagnostics \
  -H "Authorization: Bearer viewer-token"
```

### Propose remediation from current state

```bash
curl -X POST http://localhost:3000/api/remediation/propose \
  -H "Authorization: Bearer viewer-token"
```

### Execute approved remediation

```bash
curl -X POST http://localhost:3000/api/remediation/<proposal-id>/execute \
  -H "Authorization: Bearer operator-token"
```

### Analyze manifest risk

```bash
curl -X POST http://localhost:3000/api/risk-guard/analyze \
  -H "Authorization: Bearer viewer-token" \
  -H "Content-Type: application/json" \
  -d '{"manifest":"apiVersion: apps/v1\nkind: Deployment\n..."}'
```

### Inspect error-budget posture

```bash
curl -s http://localhost:3000/api/slo \
  -H "Authorization: Bearer viewer-token"
```

### Export incident evidence

```bash
curl -s http://localhost:3000/api/incidents/<incident-id>/evidence \
  -H "Authorization: Bearer viewer-token"
```

### Generate remediation GitOps artifact

```bash
curl -X POST http://localhost:3000/api/remediation/<proposal-id>/gitops \
  -H "Authorization: Bearer viewer-token" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Review rightsizing recommendations

```bash
curl -s http://localhost:3000/api/rightsizing \
  -H "Authorization: Bearer viewer-token"
```

## Environment keys that affect API behavior

- Runtime/security: `APP_MODE`, `DEV_MODE`, `WRITE_ACTIONS_ENABLED`
- Auth: `AUTH_ENABLED`, `AUTH_TOKENS`, `AUTH_PROVIDER`, `AUTH_OIDC_*`, `AUTH_TRUSTED_PROXY_CIDRS`
- Rate limits: `RATE_LIMIT_ENABLED`, `RATE_LIMIT_REQUESTS`, `RATE_LIMIT_WINDOW_SECONDS`
- Predictor: `PREDICTOR_BASE_URL`, `PREDICTOR_SHARED_SECRET`, `PREDICTOR_MODEL_PATH`
- Assistant/RAG: `ASSISTANT_*`, `OLLAMA_*`
- Alerts: `ALERTMANAGER_WEBHOOK_URL`, `SLACK_WEBHOOK_URL`, `PAGERDUTY_*`
- ChatOps: `CHATOPS_*`

## Contract source of truth

For exact schemas/status codes, use:

- `backend/internal/httpapi/openapi.yaml`
- `src/lib/api/generated/openapi-contract.ts` (generated frontend route contract; run `npm run generate:api-contract`)
