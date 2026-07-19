# Backup And Restore Runbook

Use this for planned backups and restore drills for KubeLens operational data.

## Data Stores

- Postgres stores incidents, remediations, postmortems, alert lifecycle, Ghost simulations, SQL memory, and SQL audit entries.
- SQLite stores the same SQL tables when `DATABASE_DRIVER=sqlite`.
- File memory exists only when `MEMORY_STORE=file`.
- File audit exists only when `AUDIT_STORE=file`.

## Backup

1. Confirm `GET /api/readiness/production` has no storage blockers.
2. For Postgres, run a logical backup with your platform standard, for example `pg_dump`.
3. For SQLite, stop writes, copy `DB_PATH`, then restart.
4. If file memory or file audit are enabled, copy `MEMORY_FILE_PATH` and `AUDIT_LOG_FILE`.
5. Record image tag, `APP_VERSION`, `APP_COMMIT`, and migration settings with the backup.

## Restore Drill

1. Restore into an isolated namespace or database.
2. Start KubeLens with `DATABASE_MIGRATIONS_AUTO=false` first if schema drift is suspected.
3. Verify `/api/runtime`, `/api/readiness/production`, `/api/incidents`, `/api/remediation`, `/api/ghost/simulations`, `/api/memory/runbooks`, and `/api/audit`.
4. Verify a recent audit entry with `/api/audit/{id}/verify`.

## Rollback

If restored data fails verification, stop the deployment, restore the previous backup, and keep write actions disabled until audit verification succeeds.
