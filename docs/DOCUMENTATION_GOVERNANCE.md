# Documentation Governance

This policy keeps KubeLens documentation current, reviewable, and release-ready.
Documentation review refresh: 2026-05-07 (governance process unchanged).

## 1) Scope

The following docs are in mandatory governance scope:

- `ARCHITECTURE.md`
- `FEATURES.md`
- `api.md`
- `SECURITY.md`
- `THREAT_MODEL.md`
- `OPERATIONS_VERIFICATION.md`
- `SUPPLY_CHAIN_POLICY.md`
- `SECRET_ROTATION_RUNBOOK.md`
- `IMPLEMENTATION_PROGRAM.md`

## 2) Update triggers

Documentation updates are required in the same change when:

1. API behavior, contracts, or auth/security posture changes.
2. Operational controls, deployment flow, or release process changes.
3. User-facing features, routes, views, or workflow behavior changes.
4. New threats/abuse paths are discovered or controls are added/removed.
5. Program milestone status or execution gates change for roadmap epics.

## 3) Review cadence

- Per change: docs updated in the same PR/commit as behavior changes.
- Weekly: stale-doc monitoring workflow runs and reports drift.
- Quarterly: full documentation review by maintainers.
- After incidents: update relevant runbooks/policies as part of postmortem closure.

## 4) Ownership

- Primary owner: maintainers for affected subsystem.
- Security owner: validates `SECURITY.md`, `THREAT_MODEL.md`, supply-chain and secret controls.
- Release owner: validates release/deployment doc accuracy before tag publication.

## 5) CI and automation controls

- `npm run verify:docs` validates mandatory doc links and key control references.
- `npm run verify:doc-impact` enforces docs updates for high-impact code/configuration changes.
- CI workflow enforces both checks on pushes and pull requests.
- Scheduled docs governance workflow checks staleness and opens/updates tracking issues.

## 6) Definition of done

A change is documentation-complete when:

1. Related docs are updated and committed.
2. `verify:docs` passes.
3. Security/operations implications are reflected in `SECURITY.md` and `OPERATIONS_VERIFICATION.md` when applicable.
4. Threat model is updated for new high-risk behavior.
5. `IMPLEMENTATION_PROGRAM.md` is updated when epic status or phase scope changes.

## 7) Related docs

- [SECURITY.md](SECURITY.md)
- [THREAT_MODEL.md](THREAT_MODEL.md)
- [OPERATIONS_VERIFICATION.md](OPERATIONS_VERIFICATION.md)
- [SUPPLY_CHAIN_POLICY.md](SUPPLY_CHAIN_POLICY.md)
- [SECRET_ROTATION_RUNBOOK.md](SECRET_ROTATION_RUNBOOK.md)
- [IMPLEMENTATION_PROGRAM.md](IMPLEMENTATION_PROGRAM.md)
