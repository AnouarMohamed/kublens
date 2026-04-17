# Implementation Program

This document is the execution contract for delivering all planned platform improvements while preserving code quality, architecture integrity, documentation freshness, and security posture.

## Program goals

1. Deliver all six strategic capabilities:
   - SLO and error-budget control center
   - GitOps remediation mode
   - Cost and rightsizing advisor
   - Policy preflight in Risk Guard
   - Incident replay and evidence bundles
   - Assistant quality and evaluation dashboard
2. Keep architecture clean and modular as scope grows.
3. Keep security controls at least as strong as current baseline at every milestone.
4. Keep docs continuously synced with behavior and configuration changes.

## Execution phases

### Phase 0: Guardrails (mandatory before feature merges)

- CI enforces documentation structural and impact checks.
- CI enforces dependency vulnerability audits across Go, npm, and Python.
- Toolchain/runtime baselines stay on patched releases.
- Every feature PR includes test updates, security impact notes, and docs updates.

Exit criteria:

- `verify:docs` passes in CI.
- `verify:doc-impact` passes in CI.
- Security audit steps pass in CI.

### Phase 1: Foundation epics

1. SLO and error-budget control center
2. Policy preflight in Risk Guard
3. Incident replay and evidence bundles

Exit criteria:

- API contracts updated and verified via `verify:openapi` and `verify:api-contract`.
- Feature docs added/updated in `docs/FEATURES.md`, `docs/api.md`, and operational docs where needed.
- Security review complete for new alerting and evidence export paths.

### Phase 2: Automation and governance epics

1. GitOps remediation mode
2. Cost and rightsizing advisor

Exit criteria:

- Full audit trace from proposal to execution artifact.
- Rollback and failure-path tests for every automated action path.
- Threat model updates for new trust boundaries and third-party integrations.

### Phase 3: Intelligence quality epic

1. Assistant quality and evaluation dashboard

Exit criteria:

- Retrieval quality metrics are persisted and queryable.
- Feedback loop controls are documented and tested.
- Safety checks and prompt/grounding controls have regression coverage.

## Cross-cutting quality contract

Every epic and sub-feature must satisfy all checks below before merge:

1. Code quality
   - Module boundaries respected (`npm run lint` and structure checks pass).
   - No dead paths, unclear coupling, or untested critical branches.
2. Testing
   - Unit/integration tests for backend/frontend/predictor changes.
   - E2E coverage for user-facing workflows where behavior changes.
3. Security
   - Auth/RBAC implications reviewed and tested.
   - Audit log coverage confirmed for critical operations.
   - Dependency and supply-chain checks pass.
4. Documentation
   - User-facing behavior/configuration changes reflected in docs in the same PR.
   - `docs/FEATURES.md`, `docs/api.md`, and relevant runbooks/policies updated as needed.
   - Threat/security docs updated for high-risk changes.

## Program operating rhythm

- Weekly: review milestone status and unresolved risks.
- Per PR: enforce quality, security, and docs checklists.
- Per release: re-validate supply-chain, operations verification, and rollback guidance.

## Status tracker

| Epic                                       | Status  | Owner | Target phase |
| ------------------------------------------ | ------- | ----- | ------------ |
| SLO and error-budget control center        | planned | TBD   | Phase 1      |
| Policy preflight in Risk Guard             | planned | TBD   | Phase 1      |
| Incident replay and evidence bundles       | planned | TBD   | Phase 1      |
| GitOps remediation mode                    | planned | TBD   | Phase 2      |
| Cost and rightsizing advisor               | planned | TBD   | Phase 2      |
| Assistant quality and evaluation dashboard | planned | TBD   | Phase 3      |
