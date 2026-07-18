# Threat Model

This document captures high-risk abuse paths and implemented controls for the current KubeLens release.

## Scope

- Frontend UI (`src/`)
- Backend API and middleware (`backend/internal/httpapi`)
- Cluster integration (`backend/internal/cluster`)
- Intelligence/assistant/predictor/alerts integrations

## Critical assets

- Kubernetes credentials and context payloads (`KUBECONFIG_DATA`, `KUBECONFIG_CONTEXTS`)
- Cluster resource manifests and write operations
- Session/auth state and role assignments
- Audit integrity
- Incident/remediation/postmortem workflow records
- SQL database credentials and durable workflow records

## Primary trust boundaries

1. Browser -> API (`/api/*`)
2. API -> Kubernetes API
3. API -> external systems (predictor, assistant provider, alert channels, ChatOps)

## Abuse cases and controls

| Threat                                       | Risk                                    | Controls                                                                                                          |
| -------------------------------------------- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Missing/forged auth token                    | Unauthorized read/write access          | Auth middleware, token/session validation, route role checks                                                      |
| Privilege escalation                         | Viewer bypasses operator/admin policy   | Deterministic route-to-role mapping in RBAC policy                                                                |
| Write misuse                                 | High-impact cluster mutations           | Global write gate + role checks on mutating routes                                                                |
| CSRF on cookie-auth writes                   | Cross-site mutation                     | Same-origin `Origin`/`Referer` checks for cookie-auth writes                                                      |
| Cross-origin WebSocket upgrade               | Event stream/session abuse              | Same-origin/trusted-origin check on `/api/stream/ws`                                                              |
| Rate-limit bypass                            | API exhaustion                          | Central limiter on `/api/*` requests                                                                              |
| Proxy-header spoofing                        | Rate-limit and audit evasion            | Trusted proxy CIDR allowlist for `X-Forwarded-For` trust                                                          |
| Remediation self-approval/execution in prod  | Separation-of-duties failure            | Four-eyes enforcement in remediation execute path (`prod`)                                                        |
| Unsafe or misleading GitOps artifact output  | Bad patch/advisory enters change flow   | Deterministic artifact builders, persisted artifacts, audit trail, human review                                   |
| Autonomous remediation overreach             | Proposal spam or unsafe action path     | Disabled-by-default feature gate, operator role, write gate, proposal-only output, human approval before execute  |
| Experimental telemetry privacy leakage       | Sensitive node/workload signal exposure | Disabled-by-default eBPF gate, explicit experimental labels, deployment-specific privacy/runbook review required  |
| Fleet drift false positives                  | Wrong-cluster correction pressure       | Drift report is read-only/proposal-only and limited to configured contexts with visible baseline                  |
| Prompt/knowledge injection in assistant flow | Misleading recommendations              | Deterministic context backbone, optional references, explicit source attribution                                  |
| Webhook integration abuse                    | Exfiltration or spam                    | Explicitly configured webhook endpoints and auth-gated dispatch/test endpoints                                    |
| Audit poisoning                              | Forensics degradation                   | Structured audit schema, bounded storage, sanitized fields, hash-chain verification, and optional HMAC signatures |
| Cluster context confusion                    | Wrong-cluster operations                | Explicit context selection API and visible active context in UI                                                   |
| Unsigned release artifact                    | Supply-chain compromise                 | Signed image digests + SBOM attestations with release policy enforcement                                          |
| Documentation drift                          | Unsafe/inaccurate operations            | CI docs verification + scheduled staleness checks with governance workflow                                        |

## Current assumptions and non-goals

- Full OAuth browser redirect flow is out of scope (OIDC/JWT bearer validation is supported).
- Audit signatures require `AUDIT_SIGNING_KEY`; unsigned historical entries remain verifiable only by hash chain.
- Secret-management backends are deployment-specific and external to this codebase.
- Experimental eBPF telemetry uses compatibility reports until a production node agent rollout, privacy review, and rollback runbook are complete.

## Verification references

- `backend/internal/httpapi/security_test.go`
- `backend/internal/httpapi/auth_audit_test.go`
- `backend/internal/httpapi/handlers_ops_test.go`
- `backend/internal/httpapi/contract_test.go`
- `docs/OPERATIONS_VERIFICATION.md`
- `docs/SUPPLY_CHAIN_POLICY.md`
- `docs/SECRET_ROTATION_RUNBOOK.md`
- `docs/DOCUMENTATION_GOVERNANCE.md`
