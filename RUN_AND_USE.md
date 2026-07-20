# Run, Use, Deploy, and GitHub Workflow Guide

This is the full operator and contributor guide for KubeLens AI.

## Screenshot gallery

| UI 1                                                                | UI 2                                                                | UI 3                                                                |
| ------------------------------------------------------------------- | ------------------------------------------------------------------- | ------------------------------------------------------------------- |
| ![KubeLens UI 1](Screenshots/Screenshot%202026-03-13%20235520.png)  | ![KubeLens UI 2](Screenshots/Screenshot%202026-03-13%20235534.png)  | ![KubeLens UI 3](Screenshots/Screenshot%202026-03-13%20235552.png)  |
| ![KubeLens UI 4](Screenshots/Screenshot%202026-03-13%20235600.png)  | ![KubeLens UI 5](Screenshots/Screenshot%202026-03-13%20235615.png)  | ![KubeLens UI 6](Screenshots/Screenshot%202026-03-13%20235627.png)  |
| ![KubeLens UI 7](Screenshots/Screenshot%202026-03-13%20235723.png)  | ![KubeLens UI 8](Screenshots/Screenshot%202026-03-13%20235802.png)  | ![KubeLens UI 9](Screenshots/Screenshot%202026-03-13%20235824.png)  |
| ![KubeLens UI 10](Screenshots/Screenshot%202026-03-13%20235843.png) | ![KubeLens UI 11](Screenshots/Screenshot%202026-03-13%20235859.png) | ![KubeLens UI 12](Screenshots/Screenshot%202026-03-13%20235921.png) |
| ![KubeLens UI 13](Screenshots/Screenshot%202026-03-13%20235943.png) | ![KubeLens UI 14](Screenshots/Screenshot%202026-03-13%20235957.png) | ![KubeLens UI 15](Screenshots/Screenshot%202026-03-14%20000100.png) |

## 1) Prerequisites

- Node.js 20+
- npm 10+
- Go 1.25+ (for backend tests/local backend)
- Python 3.12+ (for predictor tests)
- Docker Desktop (for image/compose workflows)
- `kubectl` (for Kubernetes deploy/use)
- Optional: `kustomize`, `helm`

## 2) Install and run locally

```bash
npm install
npm run dev
```

Endpoints:

- Frontend UI: `http://localhost:5173`
- Backend API root: `http://localhost:3000/api`
- OpenAPI: `http://localhost:3000/api/openapi.yaml`

Default behavior is safe demo mode: read-focused, no write operations.

## 3) Configuration basics

Copy base config:

```bash
cp .env.example .env
```

Core variables:

```text
APP_MODE=demo                    # dev | demo | prod
DEV_MODE=false
KUBECONFIG_DATA=
KUBECONFIG_CONTEXTS=
AUTH_ENABLED=false
AUTH_TOKENS=
WRITE_ACTIONS_ENABLED=false
PREDICTOR_BASE_URL=
PREDICTOR_SHARED_SECRET=
ASSISTANT_PROVIDER=none
ASSISTANT_API_BASE_URL=
ASSISTANT_API_KEY=
ASSISTANT_MODEL=
ASSISTANT_RAG_ENABLED=true
```

Recommended profile defaults:

- `demo`: UI demos, mock/safe behavior, no writes.
- `dev`: engineering workflow, optional auth, optional write testing.
- `prod`: authentication required, static tokens must be 32+ characters unless OIDC is used, SQL memory/audit stores and `AUDIT_SIGNING_KEY` are required, read-only by default.

## 4) Connect to real Kubernetes clusters

Single cluster via base64 kubeconfig:

PowerShell:

```powershell
$bytes = [System.IO.File]::ReadAllBytes("$HOME\.kube\config")
$env:KUBECONFIG_DATA = [Convert]::ToBase64String($bytes)
npm run dev
```

Linux/macOS:

```bash
export KUBECONFIG_DATA=$(base64 -w 0 ~/.kube/config)
npm run dev
```

Multi-cluster:

```text
KUBECONFIG_CONTEXTS=prod:<base64>,staging:<base64>,dev:<base64>
```

Verification:

```bash
kubectl cluster-info
kubectl get nodes
kubectl top nodes
kubectl top pods -A
```

If `kubectl top` fails, install/fix Metrics Server before expecting full metrics in the UI.

## 5) Enable protected write actions safely

Use auth + explicit write gate together.

```text
APP_MODE=dev
DEV_MODE=true
AUTH_ENABLED=true
AUTH_TOKENS=viewer:viewer:viewer-token,operator:operator:operator-token,admin:admin:admin-token
WRITE_ACTIONS_ENABLED=true
```

Protected write examples:

- Pod restart/delete/create
- Deployment scale/restart/rollback/apply
- Node cordon/uncordon/drain
- Remediation proposal execution

If either auth or write gate is not enabled, writes are blocked.

## 6) How to use the product (feature walkthrough)

Suggested operator path:

1. Start on **Dashboard** for cluster health and risk overview.
2. Open **Diagnostics** for deterministic issues/evidence/recommendations.
3. Use **Nodes**, **Pods**, and **Deployments** to inspect and act.
4. Create/track **Incidents** and remediation approvals.
5. Use **Memory**, **Playbooks**, and **Postmortems** to capture learning.
6. Use **Assistant** for guided troubleshooting with deterministic context.

Major areas and expected actions:

- **Dashboard**: KPIs, utilization trends, restart hotspots.
- **Diagnostics**: investigate findings with evidence and severity.
- **Predictions**: see risk-scored incident candidates.
- **Pods**: view logs/events and perform controlled pod actions.
- **Nodes**: maintenance workflows including drain previews/execution.
- **Deployments**: scale/restart/rollback and inspect rollout health.
- **Events + Audit**: follow live and historical operational traces.
- **Incidents + Remediation**: triage, approve, execute, and close loops.
- **Risk Guard**: evaluate manifest risk before apply.
- **Shift Brief**: handoff-ready operations summary.
- **Resource Catalog**: deep inventory across apps/network/storage/RBAC.

Detailed feature reference: [docs/FEATURES.md](docs/FEATURES.md)

## 7) Optional services (Predictor, Assistant, Alerts)

### Predictor service

```bash
npm run docker:build:predictor
npm run docker:run:predictor
```

```text
PREDICTOR_BASE_URL=http://localhost:8001
PREDICTOR_SHARED_SECRET=your-shared-secret
```

If predictor is unavailable, backend falls back to deterministic local prediction logic.

### Assistant + RAG

OpenAI-compatible provider example:

```text
ASSISTANT_PROVIDER=openai_compatible
ASSISTANT_API_BASE_URL=https://api.openai.com/v1
ASSISTANT_API_KEY=...
ASSISTANT_MODEL=gpt-4o
ASSISTANT_RAG_ENABLED=true
```

Local Ollama example:

```bash
ollama pull llama3.2
ollama pull nomic-embed-text
```

```text
ASSISTANT_PROVIDER=openai_compatible
ASSISTANT_API_BASE_URL=http://localhost:11434/v1
ASSISTANT_API_KEY=ollama
ASSISTANT_MODEL=llama3.2
ASSISTANT_EMBEDDING_MODEL=nomic-embed-text
ASSISTANT_EMBEDDING_BASE_URL=http://localhost:11434/v1
```

### Alerts and ChatOps

```text
ALERTMANAGER_WEBHOOK_URL=
SLACK_WEBHOOK_URL=
PAGERDUTY_ROUTING_KEY=
CHATOPS_SLACK_WEBHOOK_URL=
CHATOPS_NOTIFY_INCIDENTS=true
CHATOPS_NOTIFY_REMEDIATIONS=true
CHATOPS_NOTIFY_POSTMORTEMS=true
CHATOPS_NOTIFY_ASSISTANT_FINDINGS=false
```

## 8) Local quality gates before deploy or PR

```bash
npm run lint
npm run test:web
npm run test:go
npm run test:predictor
npm run test:e2e
npm run build
```

Backend CI parity command:

```bash
npm run ci:backend
```

## 9) Deployment guide

### Option A: Docker Compose (quick local runtime)

```bash
npm run docker:up
npm run docker:down
```

Use this when you want a local packaged runtime quickly.

### Option B: Kubernetes with Kustomize (recommended for clusters)

Available overlays:

- `k8s/overlays/dev`
- `k8s/overlays/demo`
- `k8s/overlays/prod`
- `k8s/overlays/tracing`
- `k8s/overlays/observability`

Deploy:

```bash
kubectl apply -k k8s/overlays/dev
kubectl apply -k k8s/overlays/demo
kubectl apply -k k8s/overlays/prod
kubectl apply -k k8s/overlays/tracing
kubectl apply -k k8s/overlays/observability
```

Production secret flow:

```bash
cp k8s/secret.example.yaml k8s/secret.yaml
# fill values: AUTH_TOKENS or OIDC settings, DATABASE_URL, AUDIT_SIGNING_KEY, and KUBECONFIG_DATA
# generate static tokens with a secure source such as: openssl rand -hex 32
kubectl apply -f k8s/secret.yaml
kubectl apply -k k8s/overlays/prod
```

Post-deploy verification:

```bash
kubectl -n kubernetes-operations-dashboard get pods
kubectl -n kubernetes-operations-dashboard get svc
kubectl -n kubernetes-operations-dashboard logs deploy/k8s-ops-dashboard --tail=100
```

Port-forward examples:

```bash
# app
kubectl -n kubernetes-operations-dashboard port-forward svc/k8s-ops-dashboard 3000:3000
# jaeger (if tracing overlay)
kubectl -n kubernetes-operations-dashboard port-forward svc/k8s-ops-jaeger 16686:16686
# grafana (if observability overlay)
kubectl -n kubernetes-operations-dashboard port-forward svc/k8s-ops-grafana 3001:3000
```

Update and rollback:

```bash
# apply new manifest version
kubectl apply -k k8s/overlays/prod
# rollback deployment
kubectl -n kubernetes-operations-dashboard rollout undo deploy/k8s-ops-dashboard
kubectl -n kubernetes-operations-dashboard rollout status deploy/k8s-ops-dashboard
```

Uninstall:

```bash
kubectl delete -k k8s/overlays/prod
```

More deployment details: [k8s/README.md](k8s/README.md)

### Option C: Helm

Install:

```bash
helm install kubelens ./helm/kubelens
```

Upgrade:

```bash
helm upgrade kubelens ./helm/kubelens
```

Rollback and uninstall:

```bash
helm rollback kubelens <REVISION>
helm uninstall kubelens
```

## 10) GitHub workflow (branches, PRs, CI, releases)

Daily contributor flow:

1. Sync main branch.
2. Create feature/fix branch.
3. Make focused changes and update docs.
4. Run local quality gates.
5. Commit with clear scope.
6. Open PR and pass CI.
7. Merge after review.

Commands:

```bash
git checkout main
git pull origin main
git checkout -b feat/<short-topic>
git add .
git commit -m "feat: short summary"
git push -u origin feat/<short-topic>
```

PR checklist:

- Behavior validated locally.
- `README.md` and/or docs updated if behavior/config changed.
- `docs/FEATURES.md` updated for user-facing changes.
- Tests added/updated for risky behavior.
- Changelog updated when required by release policy.

Current CI workflow (`.github/workflows/ci.yml`) runs:

- Release discipline checks (`verify:release`, `verify:changelog`, `verify:openapi`)
- Frontend lint/test/build
- Backend CI + focused package coverage
- Predictor lint/tests
- E2E suite (Playwright Chromium + Firefox)
- Kustomize + kubeconform manifest validation
- Security checks (Trivy, Hadolint)
- Docker image build + smoke tests

Release + CD workflow (`.github/workflows/release-supply-chain.yml`) runs:

- On release tags (`v*`): build/push signed dashboard + predictor images, generate SBOM attestations, then deploy to `dev` and `staging` via Helm.
- On manual dispatch: deploy an existing tag to a selected environment (`dev`, `staging`, or `prod`).
- Production deployments are controlled through GitHub Environment protections/approvals.

Required GitHub Environment secret per deploy target:

- `KUBE_CONFIG_B64` (base64 kubeconfig for that environment)

Default Helm targets in auto-CD:

- `dev`: namespace `kubernetes-operations-dashboard-dev`, release `kubelens`
- `staging`: namespace `kubernetes-operations-dashboard-staging`, release `kubelens`

Release hygiene:

- Keep `package.json` version, Docker tags, and manifests aligned.
- Keep `CHANGELOG.md` updated.
- Do not bypass failing CI on protected branches.

## 11) Operational API endpoints

- Liveness: `GET /api/healthz`
- Readiness: `GET /api/readyz`
- Runtime status: `GET /api/runtime`
- OpenAPI contract: `GET /api/openapi.yaml`
- JSON telemetry: `GET /api/metrics`
- Prometheus telemetry: `GET /api/metrics/prometheus`
- Streams: `GET /api/stream` (SSE), `GET /api/stream/ws` (WebSocket)

Full API details: [docs/api.md](docs/api.md)

## 12) Troubleshooting

- `403` on writes: role or `WRITE_ACTIONS_ENABLED` is blocking the action.
- `401` predictor calls: `PREDICTOR_SHARED_SECRET` mismatch between backend and predictor.
- Startup fails in `prod`: auth config is incomplete.
- Metrics show `N/A`: Metrics Server missing or unhealthy.
- Assistant blank/error responses: check provider, key, model, and base URL.
- Kubernetes deploy issues: render manifests locally first with `kubectl kustomize`.

## 13) Additional docs

- [README.md](README.md)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/FEATURES.md](docs/FEATURES.md)
- [docs/api.md](docs/api.md)
- [docs/SECURITY.md](docs/SECURITY.md)
- [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md)
- [docs/OPERATIONS_VERIFICATION.md](docs/OPERATIONS_VERIFICATION.md)
