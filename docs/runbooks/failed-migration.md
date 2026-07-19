# Failed Migration Runbook

Use this when startup logs report schema migration failures or `/api/readiness/production` reports `database-migrations`.

## Immediate Action

1. Keep the new deployment out of service.
2. Disable write actions if any old pod remains live.
3. Capture startup logs and the database error.
4. Confirm the database URL/path and credentials are correct.

## Triage

- For Postgres, check connectivity, permissions, locks, and schema ownership.
- For SQLite, check the mounted volume path, filesystem permissions, and disk space.
- Compare the running image tag with the expected release.
- Confirm whether `DATABASE_MIGRATIONS_AUTO=true` is intended for this environment.

## Recovery

1. If the failure is configuration, fix env/secrets and restart.
2. If the failure is a lock or transient database issue, clear the condition and restart one pod.
3. If the schema is incompatible, roll back to the last known-good image and restore from backup if needed.
4. Recheck `/api/readiness/production` before enabling traffic.
