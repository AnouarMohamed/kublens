import fs from "node:fs";
import path from "node:path";
import process from "node:process";

const root = process.cwd();

const SEMVER_PATTERN = /^\d+\.\d+\.\d+$/;

function readFile(relPath) {
  return fs.readFileSync(path.join(root, relPath), "utf8");
}

function fail(message) {
  console.error(`release-check: ${message}`);
  process.exit(1);
}

function assertPattern(content, pattern, description) {
  if (!pattern.test(content)) {
    fail(`missing or mismatched ${description}`);
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

const pkgPath = path.join(root, "package.json");
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
const version = String(pkg.version ?? "").trim();

if (!SEMVER_PATTERN.test(version)) {
  fail(`package.json version must be strict semver (x.y.z), got "${version}"`);
}

const releaseTag = `v${version}`;
const escapedTag = escapeRegExp(releaseTag);

const dockerfile = readFile("Dockerfile");
assertPattern(dockerfile, new RegExp(`^ARG APP_VERSION=${escapedTag}$`, "m"), "Dockerfile ARG APP_VERSION");

const compose = readFile("docker-compose.yml");
assertPattern(
  compose,
  new RegExp(`image:\\s*kubernetes-operations-dashboard:${escapedTag}`),
  "docker-compose dashboard image tag",
);
assertPattern(compose, new RegExp(`image:\\s*k8s-ops-predictor:${escapedTag}`), "docker-compose predictor image tag");
assertPattern(
  compose,
  new RegExp(`image:\\s*kubelens-ghost-engine:${escapedTag}`),
  "docker-compose Ghost Engine image tag",
);

const k8sDeployment = readFile("k8s/base/deployment.yaml");
assertPattern(
  k8sDeployment,
  new RegExp(`image:\\s*ghcr\\.io/anouarmohamed/kublens:${escapedTag}`),
  "k8s dashboard image tag",
);

const k8sPredictor = readFile("k8s/base/predictor-deployment.yaml");
assertPattern(
  k8sPredictor,
  new RegExp(`image:\\s*ghcr\\.io/anouarmohamed/kublenspredictor:${escapedTag}`),
  "k8s predictor image tag",
);

const k8sGhostEngine = readFile("k8s/base/ghost-engine-deployment.yaml");
assertPattern(
  k8sGhostEngine,
  new RegExp(`image:\\s*ghcr\\.io/anouarmohamed/kublensghost:${escapedTag}`),
  "k8s Ghost Engine image tag",
);

const dockerBuildScript = String(pkg.scripts?.["docker:build"] ?? "");
if (!dockerBuildScript.includes(`:${releaseTag}`)) {
  fail(`package.json script docker:build must tag image with ${releaseTag}`);
}
const dockerBuildPredictorScript = String(pkg.scripts?.["docker:build:predictor"] ?? "");
if (!dockerBuildPredictorScript.includes(`:${releaseTag}`)) {
  fail(`package.json script docker:build:predictor must tag image with ${releaseTag}`);
}
const dockerBuildGhostScript = String(pkg.scripts?.["docker:build:ghost"] ?? "");
if (!dockerBuildGhostScript.includes(`:${releaseTag}`)) {
  fail(`package.json script docker:build:ghost must tag image with ${releaseTag}`);
}

if (process.env.GITHUB_REF_TYPE === "tag") {
  const refName = String(process.env.GITHUB_REF_NAME ?? "").trim();
  if (refName !== releaseTag) {
    fail(`git tag "${refName}" must match package version tag "${releaseTag}"`);
  }
}

console.log(`release-check: OK (${releaseTag})`);
