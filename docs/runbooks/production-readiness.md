# Production Readiness Runbook

Use this before promoting a shared or customer-facing KubeLens deployment.

## Check

```bash
curl -s http://localhost:3000/api/readiness/production | jq
```

The response should be `ready` or `degraded`. `blocked` means the pod should not receive production traffic.

## Required Baseline

- `APP_MODE=prod`
- Real cluster kubeconfig is mounted through `KUBECONFIG_DATA`
- `AUTH_ENABLED=true`
- `WRITE_ACTIONS_ENABLED` is either `false` or guarded by authenticated operator/admin roles
- Durable SQL storage is active
- `DATABASE_MIGRATIONS_AUTO=true`, or migrations were run before startup
- `MEMORY_STORE=sql`
- `AUDIT_STORE=sql`
- `auditSinkFailures=0`

## Recommended Baseline

- `AUDIT_SIGNING_KEY` is configured
- Predictor service is enabled and `/api/predictor/model` has no stale or load errors
- Ghost engine is configured with `GHOST_ENGINE_ADDR`
- Alert routing is enabled for at least one channel
- Experimental feature gates are disabled unless rollout approval exists

## Escalation

If readiness is `blocked`, keep the deployment out of service and follow the issue recommendations in the response. Recheck `/api/runtime`, `/api/metrics/prometheus`, and `/api/audit?limit=20` after changes.
