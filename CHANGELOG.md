# Changelog

All notable changes to this project are documented here.

## v0.4.0

### Added

- Incident Commander with in-memory incident store, timeline/runbook generation, strict step-transition rules, resolve flow, and audited mutating actions.
- Safe Auto-Remediation with proposal generation, approval/reject/execute workflow, production four-eyes enforcement, and audited execution paths.
- Cluster Memory with atomic file-backed runbook/fix persistence, assistant team-runbook context injection, and usage-based retrieval.
- Change Risk Guard with 10 manifest checks, score/level reporting, dedicated analysis endpoint, and pre-apply force gate integration for risky YAML updates.
- Postmortem Generator with deterministic incident summaries, optional AI root-cause/prevention enrichment, conflict-safe one-postmortem-per-incident handling, and list/detail APIs.
- ChatOps Slack integration using Block Kit notifications, 5-minute dedupe rate limiting, async non-blocking dispatch, and configurable notification toggles.
- New frontend operational views for incidents, remediation, memory, risk guard, and postmortems, fully wired to typed API contracts.

### Changed

- Assistant context now includes top team runbooks from memory search on every query and increments usage counters for surfaced entries.
- Resource YAML apply now supports risk-guard `202 Accepted` responses requiring explicit `force=true` override for high-risk manifests.
- Backend HTTP contract/OpenAPI expanded to document all new operational APIs and schemas.
- Release metadata bumped to `v0.4.0` across package, Docker, Compose, Helm, and Kubernetes manifests.

### Fixed

- Resolved Prettier formatting style issues in documentation and scripts.
- Upgraded Go toolchain from 1.25.8 to 1.25.9 to address standard library vulnerabilities (CVE-2026-4947, CVE-2026-4946, CVE-2026-4870) in crypto/x509, crypto/tls, and html/template.
- Upgraded pytest from 8.4.1 to 9.0.3 to resolve CVE-2025-71176 in predictor dev dependencies.
- Upgraded google.golang.org/grpc from 1.79.2 to 1.80.0 to resolve CVE-2026-33186 (authorization bypass in HTTP/2 path validation).
- Upgraded OpenTelemetry (go.opentelemetry.io/otel\*) from 1.42.0 to v1.43.1-0.20260417155231-ac9a33d214ec to resolve CVE-2026-39883 in otel/sdk.
- Upgraded github.com/go-jose/go-jose/v4 from 4.1.3 to 4.1.4 to resolve CVE-2026-34986 (DoS via crafted JWE JSON).
- Removed the temporary Trivy ignore for CVE-2026-39883 after applying the OpenTelemetry upstream patched commit.

## v0.3.0

### Added

- End-to-end OpenTelemetry tracing across backend API, Kubernetes client calls, and predictor service continuation.
- Tracing overlay hardening with explicit Jaeger OTLP egress policies and production overlay secretKeyRef wiring for sensitive env vars.
- Predictor telemetry startup safeguards and expanded predictor unit coverage for node scoring and metric parsing paths.

### Changed

- Dashboard and pods UI were refined into a dense terminal-forward style with sharper status signaling and safer destructive action confirmation.
- Dashboard Pod Lifecycle Mix now uses a compact deterministic bar composition with summary metrics, and restart severity thresholds are consistent across views.
- Backend request decoding now preserves detailed JSON parse failures outside prod mode while keeping production-safe generic messages.
- SSE emission, stream snapshot sizing, and route/span naming were tightened for better reliability and observability cardinality control.
- RAG index construction now fetches source documents concurrently to reduce refresh latency under slow documentation endpoints.
- CI hardening: backend tests run with `-race`, E2E has a job timeout, and Docker build job includes post-build smoke checks.
- Release metadata bumped to `v0.3.0` across package/docker/helm/k8s artifacts.

## v0.2.0

### Added

- Authentication and role-based enforcement for dashboard API flows.
- CSRF same-origin validation for cookie-authenticated mutating routes.
- API contract tests for core endpoints, mutating action payloads, and auth error shape.
- Playwright E2E coverage for dashboard smoke and auth role matrix.
- Release/version consistency checks (`verify:release`, `verify:changelog`) in CI.
- Threat model and operations verification runbook docs.
- Runtime health/readiness endpoints and Prometheus-format metrics endpoint.
- Published OpenAPI contract endpoint (`/api/openapi.yaml`) and CI contract validation (`verify:openapi`).
- Frontend AppShell coordination split into focused hooks for view access, cluster switching, and search navigation.
- Expanded frontend hook/unit coverage for runtime/auth capability gating and context switching behavior.
- Playwright performance smoke check and CI multi-browser matrix (Chromium + Firefox on CI).

### Changed

- Audit entries now sanitize client IP representation (strip source port).
- Bootstrap auth wiring now includes header-token and trusted CSRF domain controls.
- CI pipeline now includes release discipline, OpenAPI contract checks, and stronger E2E verification.
- Kubernetes liveness/readiness probes moved to dedicated health endpoints and overlay RBAC removed default `secrets` read privilege.
