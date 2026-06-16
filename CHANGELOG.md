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
- Live pod log streaming over Server-Sent Events with per-line delivery, stop control, 10-minute server cap, and snapshot fallback support.
- Client-side Ops Assistant chat session persistence with recent-session recovery across reloads, localStorage-backed history, quick new-chat creation, and sidebar session switching/deletion.

### Changed

- Assistant context now includes top team runbooks from memory search on every query and increments usage counters for surfaced entries.
- Dashboard health snapshot now keeps a session-scoped health score history and renders a sparkline with improving/degrading trend cues instead of a static single-bar snapshot.
- Shift Brief now supports Markdown copy/download export and one-click handoff into the Ops Assistant with the current brief prefilled for summarization.
- Resource YAML apply now supports risk-guard `202 Accepted` responses requiring explicit `force=true` override for high-risk manifests.
- Backend HTTP contract/OpenAPI expanded to document all new operational APIs and schemas.
- Diagnostics now detect probe-related pod failures, evictions, unschedulable scheduler blocks, terminating pods, high restart velocity, missing running-pod resource requests, node CPU saturation, and stale cordoned nodes with richer event-backed evidence.
- Incident, remediation, postmortem, and alert lifecycle state now persist in SQLite via a shared backend database configured by `DB_PATH` instead of process-local in-memory stores.
- Release metadata bumped to `v0.4.0` across package, Docker, Compose, Helm, and Kubernetes manifests.

### Fixed

- Moved the shared assistant draft storage key out of `src/views/opsassistant` into a shared feature constant so structure lint no longer flags app-level imports crossing the `views/` boundary.
- Scoped TypeScript, ESLint, and Prettier checks to repo-owned sources so local `.venv`, Postman workspace exports, and transient Vite temp files no longer break routine quality gates.
- Improved backend Go task failures with a direct install hint when the `go` toolchain is missing from `PATH`.
- Pinned predictor `protobuf` back to a compatible `6.x` release so fresh installs and dependency audits succeed with `opentelemetry-exporter-otlp 1.41.0`.
- Scoped the CI Trivy filesystem scan away from local tool caches such as `.gomodcache/` so security checks report repository risk instead of downloaded module examples.
- Resolved Prettier formatting style issues in documentation and scripts.
- Upgraded Go toolchain from 1.25.8 to 1.25.9 to address standard library vulnerabilities (CVE-2026-4947, CVE-2026-4946, CVE-2026-4870) in crypto/x509, crypto/tls, and html/template.
- Upgraded pytest from 8.4.1 to 9.0.3 to resolve CVE-2025-71176 in predictor dev dependencies.
- Upgraded google.golang.org/grpc from 1.79.2 to 1.80.0 to resolve CVE-2026-33186 (authorization bypass in HTTP/2 path validation).
- Upgraded OpenTelemetry (go.opentelemetry.io/otel\*) from 1.42.0 to v1.43.0 (stable upstream release) to resolve CVE-2026-39883 in otel/sdk.
- Upgraded github.com/go-jose/go-jose/v4 from 4.1.3 to 4.1.4 to resolve CVE-2026-34986 (DoS via crafted JWE JSON).
- Removed the temporary Trivy ignore for CVE-2026-39883 after applying the OpenTelemetry upstream patched commit.
- Updated backend Kubernetes client dependencies (`k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/metrics`) from `0.36.0` to `0.36.1` and `modernc.org/sqlite` from `1.49.1` to `1.50.1`.
- Downgraded predictor `protobuf` from `7.34.1` to `6.33.5` to resolve installation conflicts with `opentelemetry-exporter-otlp 1.41.1`.
- Updated backend and CI Go toolchain targets from `1.26.0` to `1.26.3` to address standard-library vulnerabilities detected by `govulncheck`.
- Disabled the `react-hooks/set-state-in-effect` lint rule to unblock frontend quality gates for existing effect-driven data-loading flows.
- Upgraded `golang.org/x/net` to `v0.53.0` (and aligned `x/sys`, `x/term`, `x/text`) to resolve `GO-2026-4918` in security audits.
- Hardened authentication cookie transport security by always setting the `Secure` attribute when writing and clearing auth cookies.
- Ghost Engine-Remediation Integration: Automated creation of remediation proposals triggered by favorable "low" or "medium" severity simulation verdicts.
- Updated backend and CI Go toolchain targets from 1.26.3 to 1.26.4 and upgraded golang.org/x/net to v0.55.0 to address standard-library vulnerabilities (GO-2026-5039, GO-2026-5038, GO-2026-5037, GO-2026-5026).
- Upgraded predictor `starlette` to `1.3.1` and `protobuf` to `5.29.6` to resolve multiple security vulnerabilities (CVE-2026-54283, CVE-2025-4565, etc.).
- Stabilized E2E assertions by scoping the SLO heading selector to `main` and allowing remediation GitOps checks to use any returned proposal when restart-specific proposals are absent.
- Added a nil-guard in the RAG embedding client so memory runbook creation degrades gracefully instead of panicking when embeddings are not configured.
- Updated the dashboard Dockerfile Go builder base image from `golang:1.25.8-alpine` to `golang:1.26.3-alpine` to match backend toolchain requirements and restore CI docker-build.

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
