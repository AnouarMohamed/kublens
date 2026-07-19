# Write Gate And Rollback Runbook

Use this before enabling cluster mutations or executing remediation in production.

## Preflight

- `APP_MODE=prod`
- `AUTH_ENABLED=true`
- Operators and admins authenticate through the intended auth provider
- `WRITE_ACTIONS_ENABLED=true` only during approved change windows
- Audit entries are durable and signed
- GitOps artifact generation is tested for the target remediation type

## Enable Writes

1. Set `WRITE_ACTIONS_ENABLED=true`.
2. Restart or roll the backend deployment.
3. Confirm `/api/runtime` shows `writeActionsEnabled=true`.
4. Run a low-risk dry run or preview endpoint first.
5. Confirm audit entries include the actor and action.

## Rollback

1. Set `WRITE_ACTIONS_ENABLED=false`.
2. Roll the deployment.
3. Confirm mutating endpoints return `403`.
4. Use the stored GitOps artifact or Kubernetes rollout history to reverse the change.
5. Attach `/api/audit` evidence to the incident or change record.
