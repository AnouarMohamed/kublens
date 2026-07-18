# Implementation Program

This document is the execution contract for making KubeLens enterprise-ready as an incident and change-risk copilot while preserving code quality, architecture integrity, documentation freshness, and security posture.

## Program goals

1. Preserve the six shipped foundation capabilities:
   - SLO and error-budget control center
   - GitOps remediation mode
   - Cost and rightsizing advisor
   - Policy preflight in Risk Guard
   - Incident replay and evidence bundles
   - Assistant quality and evaluation dashboard
2. Make the primary workflow explicit: detect risk, simulate change, explain evidence, produce governed GitOps remediation, and export an incident record.
3. Keep architecture clean and modular as Ghost simulation, predictor governance, durable storage, and audit integrity mature.
4. Keep security controls at least as strong as current baseline at every milestone.
5. Keep docs continuously synced with behavior and configuration changes.

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

### Phase 1: Shipped foundation epics

1. SLO and error-budget control center
2. Policy preflight in Risk Guard
3. Incident replay and evidence bundles

Exit criteria:

- API contracts updated and verified via `verify:openapi` and `verify:api-contract`.
- Feature docs added/updated in `docs/FEATURES.md`, `docs/api.md`, and operational docs where needed.
- Security review complete for new alerting and evidence export paths.

### Phase 2: Shipped automation and governance epics

1. GitOps remediation mode
2. Cost and rightsizing advisor

Exit criteria:

- Full audit trace from proposal to execution artifact.
- Rollback and failure-path tests for every automated action path.
- Threat model updates for new trust boundaries and third-party integrations.

### Phase 3: Shipped intelligence quality epic

1. Assistant quality and evaluation dashboard

Exit criteria:

- Retrieval quality metrics are persisted and queryable.
- Feedback loop controls are documented and tested.
- Safety checks and prompt/grounding controls have regression coverage.

### Phase 4: Enterprise readiness

1. Durable storage and migrations
2. Tamper-evident critical audit records
3. Incident Risk Workbench as the primary product surface
4. Ghost simulation persistence, confidence, and scheduler-fidelity improvements
5. Predictor ML governance, shadow mode, model metadata, and evaluation gates

Exit criteria:

- Production deployments use durable SQL-backed stores. This release supports file-backed SQLite and Postgres with automatic migrations.
- Critical audit records can be verified for hash-chain integrity.
- Operators can complete the risk workflow from one workbench without losing access to drilldown views.
- Ghost and predictor outputs expose confidence, model/version metadata, and known limitations.
- Enterprise readiness checks report actionable subsystem state.

### Phase 5: Experimental deep visibility and autonomy

1. eBPF node telemetry agent
2. Fleet-wide drift detection
3. Policy-gated autonomous remediation proposal loop

Exit criteria:

- Experimental features are disabled by default, labeled as experimental in API responses, and proposal-only until production trust, rollback, and operational runbooks are complete.

## Cross-cutting quality contract

Every epic and sub-feature must satisfy all checks below before merge:

1. Code quality
   - Module boundaries respected (`npm run lint` and structure checks pass).
   - No dead paths, unclear coupling, or untested critical branches.
   - Shared frontend hooks must avoid render-phase ref mutation and keep request lifecycle behavior covered by focused tests.
2. Testing
   - Unit/integration tests for backend/frontend/predictor changes.
   - E2E coverage for user-facing workflows where behavior changes.
   - Incident and remediation UI smoke coverage verifies authenticated navigation, incident detail rendering, remediation proposal generation, and GitOps artifact rendering.
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
| SLO and error-budget control center        | shipped | TBD   | Phase 1      |
| Policy preflight in Risk Guard             | shipped | TBD   | Phase 1      |
| Incident replay and evidence bundles       | shipped | TBD   | Phase 1      |
| GitOps remediation mode                    | shipped | TBD   | Phase 2      |
| Cost and rightsizing advisor               | shipped | TBD   | Phase 2      |
| Assistant quality and evaluation dashboard | shipped | TBD   | Phase 3      |
| Durable enterprise storage                 | shipped | TBD   | Phase 4      |
| Tamper-evident audit verification          | shipped | TBD   | Phase 4      |
| Incident Risk Workbench                    | shipped | TBD   | Phase 4      |
| Ghost confidence and scheduler fidelity    | shipped | TBD   | Phase 4      |
| Predictor ML governance                    | shipped | TBD   | Phase 4      |
| eBPF deep telemetry                        | shipped | TBD   | Phase 5      |
| Fleet drift correction                     | shipped | TBD   | Phase 5      |
| Autonomous remediation                     | shipped | TBD   | Phase 5      |
