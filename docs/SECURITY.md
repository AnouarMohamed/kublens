# Security And Trust Boundaries

## Trust boundaries

- Browser/UI input is untrusted.
- Backend API is the policy enforcement boundary.
- Kubernetes credentials/context payloads are high-trust secrets.
- External providers (assistant, predictor, alert channels, ChatOps) are outbound trust boundaries.

## Runtime security controls

## Auth and authorization

- Token/session authentication with explicit role mapping (`viewer`, `operator`, `admin`)
- Route-level minimum role checks
- Optional OIDC/JWT issuer + claim mapping
- `X-Auth-Token` transport disabled by default and rejected in `prod`

## Write safety

- Global write gate: mutating cluster actions require `WRITE_ACTIONS_ENABLED=true`
- Mutating routes still require authorized role (`operator`/`admin`)
- In `prod`, remediation execution enforces four-eyes separation (approver != executor)

## Request protection

- Rate limiting on `/api/*`
- Trusted proxy CIDR allowlist (`AUTH_TRUSTED_PROXY_CIDRS`) for controlled `X-Forwarded-For` trust
- Same-origin CSRF checks for cookie-authenticated mutating requests
- Same-origin WebSocket origin checks (`/api/stream/ws`)
- Request timeout middleware on non-streaming paths
- Recovery middleware for panic containment
- Explicit HTTP security headers (`CSP`, `HSTS`, `X-Frame-Options`, `X-Content-Type-Options`)

## Audit and traceability

- Per-request audit records with actor, route, status, and latency
- Action-specific audit labels for critical operations
- Audit entries include hash-chain verification; `APP_MODE=prod` requires HMAC signatures through `AUDIT_SIGNING_KEY`
- Persisted remediation GitOps artifacts with actor/timestamp linkage for governed change review
- SQL-backed incident, remediation, postmortem, alert lifecycle, and Ghost simulation records through SQLite or Postgres
- Optional OpenTelemetry traces for backend and predictor paths

## Continuous assurance

- CodeQL SAST scanning for Go, JavaScript/TypeScript, and Python (`.github/workflows/codeql.yml`)
- Trivy + Hadolint checks in CI for filesystem/image and Dockerfile risk detection
- Go vulnerability scanning (`govulncheck`) in CI
- npm production dependency audit (`npm audit --omit=dev --audit-level=high`) in CI
- Predictor dependency vulnerability audit (`pip-audit`) in CI
- Dependabot weekly dependency update PRs for npm, Go modules, pip, and GitHub Actions
- Documentation governance checks in CI (`verify:docs` + `verify:doc-impact`) and weekly staleness monitoring

## Supply chain controls

- Release workflow signs dashboard and predictor container digests with Cosign keyless signatures
- Release workflow generates CycloneDX SBOMs and attaches signed SBOM attestations
- Release policy documented in `SUPPLY_CHAIN_POLICY.md`
- Secret rotation process documented in `SECRET_ROTATION_RUNBOOK.md`

## Deployment hardening

- Non-root containers
- Dropped Linux capabilities
- Read-only root filesystem posture in deployment overlays
- NetworkPolicy with explicit allow paths
- RBAC manifests per overlay
- PDB/HPA for availability posture

## Experimental feature controls

- eBPF telemetry, fleet drift detection, fleet drift proposal generation, and autonomous remediation proposal generation are disabled by default. Experimental eBPF telemetry ingestion and fleet drift proposal generation require operator-authenticated requests when auth is enabled.
- Experimental API responses include explicit maturity/limitation fields.
- Fleet drift proposal generation writes review-only remediation proposals from warning-level drift signals. Autonomous remediation proposal generation requires operator authorization and the global write gate; generated items remain proposals and still require human approval before execution.

## Operational recommendations

- Use `APP_MODE=prod` with `AUTH_ENABLED=true` in shared environments.
- Keep write actions disabled unless operationally required.
- Keep experimental feature flags disabled unless the environment has rollback, privacy, and review runbooks for that feature.
- Rotate static tokens and prefer OIDC/JWT where possible.
- Restrict egress to approved integrations only.
- Review audit logs regularly and alert on suspicious write attempts.

## Related docs

- [THREAT_MODEL.md](THREAT_MODEL.md)
- [OPERATIONS_VERIFICATION.md](OPERATIONS_VERIFICATION.md)
- [SUPPLY_CHAIN_POLICY.md](SUPPLY_CHAIN_POLICY.md)
- [SECRET_ROTATION_RUNBOOK.md](SECRET_ROTATION_RUNBOOK.md)
- [DOCUMENTATION_GOVERNANCE.md](DOCUMENTATION_GOVERNANCE.md)
- [api.md](api.md)
