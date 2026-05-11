# Secret Rotation Runbook

This runbook defines formal secret-rotation controls for KubeLens runtime and integrations.
Documentation review refresh: 2026-05-07 (runbook content remains current).

## 1) Scope

- Static auth tokens (`AUTH_TOKENS`)
- Predictor shared secret (`PREDICTOR_SHARED_SECRET`)
- Assistant provider key (`ASSISTANT_API_KEY`)
- Alerting and ChatOps credentials (`SLACK_WEBHOOK_URL`, `PAGERDUTY_ROUTING_KEY`, `CHATOPS_SLACK_WEBHOOK_URL`)
- Kubernetes context payload secrets (`KUBECONFIG_DATA`, `KUBECONFIG_CONTEXTS`)
- GitHub Environment deploy kubeconfigs (`KUBE_CONFIG_B64` for `dev`/`staging`/`prod`)

## 2) Control objectives

1. Limit secret lifetime and blast radius.
2. Ensure rotation is repeatable, auditable, and reversible.
3. Prevent untracked/manual secret drift between environments.

## 3) Rotation cadence

- `AUTH_TOKENS`: every 30 days (or eliminate via OIDC where possible)
- Integration secrets (Slack/PagerDuty/assistant/predictor): every 90 days
- Emergency rotation: immediately after suspected leak or unauthorized access

## 4) Roles and approvals

- Requestor: service owner/on-call operator
- Approver: security owner or platform lead
- Executor: operator with production secret update permissions
- Verifier: separate reviewer validates post-rotation health and audit evidence

## 5) Standard rotation procedure

1. Prepare:
   - inventory current secret consumers
   - generate replacement secret values in approved secret manager
   - schedule maintenance window if required
2. Stage:
   - update secret values in non-production environment
   - run smoke checks (`/api/healthz`, `/api/readyz`, auth login, predictor integration)
3. Execute:
   - apply updated Kubernetes Secret/secret manager entries in production
   - restart workloads only if required by runtime behavior
   - update GitHub Environment `KUBE_CONFIG_B64` values for automated CD targets when cluster credentials rotate
4. Validate:
   - verify auth/login behavior
   - verify predictor and alert integrations
   - confirm audit entries show expected operator actions
5. Close:
   - revoke old secret values
   - attach evidence to change record

## 6) Emergency rotation procedure

- Trigger conditions:
  - leaked token/key in logs, chat, commit, or ticket
  - suspicious authentication activity
  - compromised integration account
- Response SLA:
  - initial containment: within 30 minutes
  - full rotation and revocation: within 4 hours
- Required actions:
  - rotate all impacted secrets
  - revoke old values immediately
  - force logout/session invalidation if relevant
  - open incident and postmortem entry

## 7) Audit evidence requirements

For each rotation event, capture:

- ticket/change ID
- rotated secret category
- executor + approver identities
- start/end timestamps
- validation results
- confirmation old secret was revoked

## 8) Verification checklist

- `kubectl -n kubernetes-operations-dashboard get secret k8s-ops-secret -o yaml` (metadata only, no value disclosure)
- `GET /api/readyz` returns healthy
- auth and integration smoke tests pass
- audit log contains expected rotation-related operations

## 9) Related docs

- [SECURITY.md](SECURITY.md)
- [OPERATIONS_VERIFICATION.md](OPERATIONS_VERIFICATION.md)
- [SUPPLY_CHAIN_POLICY.md](SUPPLY_CHAIN_POLICY.md)
