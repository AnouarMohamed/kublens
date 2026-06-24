# Supply Chain Security Policy

This policy defines mandatory controls for release integrity, artifact traceability, and SBOM publication.
Documentation review refresh: 2026-05-07 (policy remains current).

## 1) Scope

- Dashboard images: `ghcr.io/<owner>/kublens`, `docker.io/<docker-username>/kublens`
- Predictor images: `ghcr.io/<owner>/kublenspredictor`, `docker.io/<docker-username>/kublenspredictor`
- Ghost Engine images: `ghcr.io/<owner>/kublensghost`, `docker.io/<docker-username>/kublensghost`
- Source repository and release tags

## 2) Mandatory controls

1. Release builds must originate from a Git tag (`v*`) on `main`.
2. Every release image must be signed (Sigstore Cosign, keyless OIDC identity).
3. Every release image must have an SBOM attached (CycloneDX JSON minimum).
4. Release artifacts must be immutable and referenced by digest.
5. CI security checks (`Trivy`, `Hadolint`, `CodeQL`, tests) must pass before release publication.
6. Dependency update automation (`Dependabot`) must remain enabled for supported ecosystems.
7. Release-tag deployments must flow through the automated CD workflow to `dev` and `staging`.
8. Production deployment must use manual workflow dispatch with environment approval controls.

## 3) Signing requirements

- Signing tool: `cosign`
- Signature mode: keyless (`--yes`, GitHub OIDC identity)
- Required signature subject: this repository's release workflow identity
- Signature target: image digests (not mutable tags)

## 4) SBOM requirements

- SBOM format: CycloneDX JSON
- Minimum scope: both dashboard and predictor images
- Publication: uploaded as workflow artifacts and attached as signed attestations
- Retention: keep release SBOM artifacts for at least 12 months

## 5) Verification requirements

Before approving release deployment:

1. Verify image signatures against expected identity.
2. Verify SBOM attestations are present for each release image digest.
3. Confirm security and test workflows are green for the release commit/tag.
4. Confirm `dev` and `staging` deploy jobs completed successfully (or were intentionally skipped due missing environment secret configuration).

## 6) Exception handling

- Any temporary bypass requires:
  - documented reason
  - approving owner
  - expiry date
  - follow-up remediation ticket
- Expired exceptions must be removed immediately.

## 7) Ownership and review cadence

- Owner: platform/security maintainers
- Review cadence: quarterly or after major pipeline/tooling changes

## 8) Related controls

- [SECURITY.md](SECURITY.md)
- [OPERATIONS_VERIFICATION.md](OPERATIONS_VERIFICATION.md)
- [REGISTRY_RELEASE.md](REGISTRY_RELEASE.md)
- [SECRET_ROTATION_RUNBOOK.md](SECRET_ROTATION_RUNBOOK.md)
- [DOCUMENTATION_GOVERNANCE.md](DOCUMENTATION_GOVERNANCE.md)
