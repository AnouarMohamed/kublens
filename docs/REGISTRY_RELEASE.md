# Registry Release Runbook

This runbook covers the first signed registry package release for KubeLens AI.

## Published images

Tagged releases publish both application images to GitHub Container Registry and Docker Hub:

- `ghcr.io/<github-owner>/kublens:<version-tag>`
- `ghcr.io/<github-owner>/kublenspredictor:<version-tag>`
- `ghcr.io/<github-owner>/kublensghost:<version-tag>`
- `docker.io/<docker-username>/kublens:<version-tag>`
- `docker.io/<docker-username>/kublenspredictor:<version-tag>`
- `docker.io/<docker-username>/kublensghost:<version-tag>`

Example for release `v0.4.2`:

- `ghcr.io/<github-owner>/kublens:v0.4.2`
- `ghcr.io/<github-owner>/kublenspredictor:v0.4.2`
- `ghcr.io/<github-owner>/kublensghost:v0.4.2`
- `docker.io/<docker-username>/kublens:v0.4.2`
- `docker.io/<docker-username>/kublenspredictor:v0.4.2`
- `docker.io/<docker-username>/kublensghost:v0.4.2`

The release workflow intentionally publishes immutable version tags only. Do not add `latest` until the release policy defines how it is promoted and rolled back.

## Required repository secrets

GitHub Container Registry publication uses the built-in `GITHUB_TOKEN`.

Docker Hub publication requires these repository secrets:

- `DOCKER_USERNAME`: Docker Hub namespace that owns the image repositories.
- `DOCKER_PASSWORD`: Docker Hub access token with read/write/delete or read/write repository permissions.

Use a Docker Hub access token, not the account password.

## Required Docker Hub repositories

Create these repositories under the Docker Hub namespace from `DOCKER_USERNAME` before the first release:

- `kublens`
- `kublenspredictor`
- `kublensghost`

If the Docker Hub account allows repository creation on push, the workflow can create them implicitly. Creating them ahead of time gives cleaner permission failures and lets the visibility be set intentionally.

## Release steps

1. Confirm CI, CodeQL, and docs governance checks are green on `main`.
2. Confirm `package.json`, image tags, Helm values, and `CHANGELOG.md` agree on the release version.
3. Confirm `DOCKER_USERNAME` and `DOCKER_PASSWORD` are configured as repository secrets.
4. Confirm Docker Hub repositories exist and are owned by `DOCKER_USERNAME`.
5. Push a release tag:

```bash
git tag v0.4.2
git push origin v0.4.2
```

Use the next release tag if `v0.4.2` already exists.

## What the workflow does

On a `v*` tag push, `.github/workflows/release-supply-chain.yml`:

1. Builds the dashboard, predictor, and Ghost Engine images once.
2. Pushes each image to GHCR and Docker Hub with the same version tag.
3. Generates CycloneDX SBOMs for all image digests.
4. Uploads SBOMs as workflow artifacts.
5. Signs GHCR and Docker Hub image digest references with keyless Cosign.
6. Attests the SBOMs to GHCR and Docker Hub image digest references.
7. Deploys the GHCR images to `dev` and then `staging` when each environment has `KUBE_CONFIG_B64`.

## Optional deployment setup

Registry publication does not require cluster credentials. Automated deployment does.

To enable deployment after image publication, add `KUBE_CONFIG_B64` as an environment secret in each GitHub environment that should deploy:

- `dev`
- `staging`
- `prod`

The value must be a base64-encoded kubeconfig with permission to install or upgrade the Helm release in the target namespace.

## Verification

After the release workflow succeeds:

1. Confirm all six image tags exist in GHCR and Docker Hub.
2. Download the `release-sboms-<version-tag>` artifact from the workflow run.
3. Verify image signatures and SBOM attestations before production promotion.
4. Confirm `dev` and `staging` deployment jobs either succeeded or were intentionally skipped because `KUBE_CONFIG_B64` is not configured.
