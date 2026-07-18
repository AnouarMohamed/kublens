# Operations Verification Runbook

Use this checklist after deployments and before enabling write features in shared environments.

## 1) Runtime posture

```bash
curl -s http://localhost:3000/api/runtime | jq
```

Verify:

- `mode` is expected (`dev`/`demo`/`prod`)
- `authEnabled=true` in production
- `writeActionsEnabled` matches intended policy
- `databaseDriver` is `sqlite` or `postgres`
- `enterpriseStorage=true` in production
- `alertsEnabled` and `ragEnabled` match configuration intent

For Postgres deployments, verify `DATABASE_DRIVER=postgres`, `DATABASE_URL` is set, and startup logs do not report migration failures.

## 2) Auth and role gating

1. Login as viewer (`POST /api/auth/login`).
2. Attempt mutating operation (for example `POST /api/pods`).
3. Expected: `403`.
4. Login as operator and repeat.
5. Expected: `200` only when `WRITE_ACTIONS_ENABLED=true`.

## 3) CSRF protection for cookie-auth writes

With a valid session cookie:

- Send mutating request with `Origin: https://evil.example`
  Expected: `403` (`cross-site request blocked`).
- Send same request with same-origin `Origin`/`Referer`
  Expected: normal auth/validation path.

## 4) Audit trail verification

```bash
curl -s "http://localhost:3000/api/audit?limit=20" | jq
```

Verify:

- Mutating actions are logged (`pod.create`, `node.cordon`, `resource.apply`, `remediation.execute`, etc.)
- Actor fields (`user`, `role`) are populated for authenticated requests
- `clientIp` is normalized and does not include source port
- No bearer tokens are persisted in log entries

## 5) Node maintenance safety checks

Test node maintenance endpoints:

- `GET /api/nodes/{name}/drain/preview`
- `POST /api/nodes/{name}/drain`
- `POST /api/nodes/{name}/cordon`
- `POST /api/nodes/{name}/uncordon`

Verify:

- Force drain requires admin role and non-empty reason
- Not-found/unsupported responses are explicit and stable

## 6) Remediation workflow checks

1. `POST /api/remediation/propose`
2. `POST /api/remediation/{id}/approve`
3. `POST /api/remediation/{id}/execute`
4. `POST /api/remediation/{id}/reject` on a different proposal

Verify:

- Execution only allowed after approval
- In `prod`, approver and executor must differ (four-eyes)
- Executed proposal links back to incidents where applicable
- `POST /api/remediation/{id}/gitops` stores a retrievable artifact at `GET /api/remediation/{id}/gitops`
- Audit log includes `remediation.gitops.generate` entries when artifacts are produced

## 7) Rightsizing advisor checks

- `GET /api/rightsizing`

Verify:

- Response contains at least one recommendation with current and recommended requests/limits
- Non-balanced recommendations include a GitOps artifact preview or advisory
- Summary totals expose reclaimable CPU/memory strings for operator review

## 8) Alerting and lifecycle checks

- `POST /api/alerts/test` or `/api/alerts/dispatch`
- `GET /api/alerts/lifecycle`
- `POST /api/alerts/lifecycle`

Verify:

- Channel dispatch result contains per-channel outcome
- Lifecycle status transitions are persisted and returned correctly

## 9) Predictor governance checks

```bash
curl -s http://localhost:3000/api/predictor/model | jq
curl -s http://localhost:8001/model | jq
curl -s http://localhost:8001/telemetry | jq
```

Verify:

- Deterministic mode is the default when no model is configured
- Shadow/blended modes report model version, metadata load state, feature requirements, evaluation metrics, and promotion gates
- Predictor telemetry updates after `GET /api/predictions?force=1`

## 10) Ghost persistence checks

1. Run `POST /api/ghost/simulations`.
2. Restart the backend using the same SQL store.
3. Call `GET /api/ghost/simulations`.

Verify the prior simulation record remains available with `topologyHash`, `confidence`, and `limitations`.

## 11) Experimental feature gates

```bash
curl -s http://localhost:3000/api/experimental | jq
curl -s http://localhost:3000/api/experimental/ebpf/nodes | jq
curl -s http://localhost:3000/api/experimental/fleet-drift | jq
```

Verify:

- All experimental features are disabled by default
- Responses include `experimental=true`
- `POST /api/experimental/autonomous-remediation/propose` requires operator role and `WRITE_ACTIONS_ENABLED=true`

## 12) API contract and backend quality checks

```bash
npm run test:go
npm run ci:backend
npm run verify:openapi
npm run verify:api-contract
```

Critical suites include:

- API contract tests
- Auth/audit/security tests
- Handler policy tests (including remediation operations)

## 13) Frontend and e2e checks

```bash
npm run lint
npm run test:web
npm run test:e2e
```

Verify:

- View navigation and search operate correctly
- Node/pod maintenance flows succeed
- Audit stream and assistant view load without regressions

## 14) Tracing verification (optional)

If OTEL export is enabled:

1. Open Jaeger and select `kubelens-backend`.
2. Trigger a prediction request and an assistant request.
3. Verify end-to-end span continuity: browser -> API -> cluster/predictor/assistant paths.

## 15) Observability overlay verification (optional)

If `k8s/overlays/observability` is installed:

1. Port-forward Grafana and Prometheus services.
2. Confirm API request rate, latency, and status panels update during active usage.

## 16) Security headers and WebSocket origin checks

- Call `GET /api/healthz` and verify response headers include:
  - `Content-Security-Policy`
  - `X-Frame-Options: DENY`
  - `X-Content-Type-Options: nosniff`
- For HTTPS traffic, verify `Strict-Transport-Security` is present.
- Attempt `/api/stream/ws` with cross-origin `Origin` and verify `403`.

## 17) Supply chain and secret-rotation controls

Before production release:

1. Confirm release artifacts are signed and SBOM attestations exist.
2. Confirm `CodeQL` scans report no new high-severity findings.
3. Confirm secret-rotation evidence is up to date for the current quarter.
4. Validate no expired security exceptions exist for signing/SBOM/rotation controls.
5. Confirm automated CD deploy jobs succeeded for `dev` and `staging` before approving production promotion.

## 18) Documentation governance controls

Before merge/release:

1. Run `npm run verify:docs`.
2. Confirm feature/security/operations changes are reflected in docs.
3. Review latest docs staleness workflow report and close/update any open governance issue.

References:

- [SUPPLY_CHAIN_POLICY.md](SUPPLY_CHAIN_POLICY.md)
- [SECRET_ROTATION_RUNBOOK.md](SECRET_ROTATION_RUNBOOK.md)
- [DOCUMENTATION_GOVERNANCE.md](DOCUMENTATION_GOVERNANCE.md)
