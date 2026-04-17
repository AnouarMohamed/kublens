import { spawnSync } from "node:child_process";

const args = process.argv.slice(2);
const explicitBase = readArgValue("--base");
const explicitHead = readArgValue("--head") ?? "HEAD";

const docsPrefixes = ["docs/"];
const docsFiles = new Set(["README.md", "RUN_AND_USE.md", "CHANGELOG.md"]);
const impactPrefixes = ["backend/cmd/", "backend/internal/", "predictor/app/", "src/", "k8s/", "helm/"];
const impactFiles = new Set([
  ".env.example",
  "Dockerfile",
  "predictor/Dockerfile",
  "docker-compose.yml",
  "package.json",
  "backend/go.mod",
  "predictor/requirements.txt",
  "predictor/requirements-dev.txt",
]);

const baseRef = resolveBaseRef(explicitBase);
const headRef = explicitHead;
const changedFiles = listChangedFiles(baseRef, headRef);

const docsChanged = changedFiles.some((filePath) => isDocsFile(filePath));
const impactingFiles = changedFiles.filter((filePath) => isImpactFile(filePath) && !isNonBehaviorChange(filePath));

if (impactingFiles.length === 0) {
  console.log("doc-impact-check: OK (no high-impact behavior/config changes detected)");
  process.exit(0);
}

if (docsChanged) {
  console.log(`doc-impact-check: OK (${impactingFiles.length} high-impact changes with documentation updates present)`);
  process.exit(0);
}

if (process.env.DOCS_IMPACT_BYPASS === "1") {
  console.warn(
    "doc-impact-check: BYPASSED via DOCS_IMPACT_BYPASS=1 (high-impact changes detected without docs updates)",
  );
  process.exit(0);
}

console.error("doc-impact-check: FAILED");
console.error(`- Compared range: ${baseRef}...${headRef}`);
console.error("- High-impact behavior/configuration changes were detected without matching docs updates.");
console.error("- Changed high-impact files:");
for (const filePath of impactingFiles) {
  console.error(`  - ${filePath}`);
}
console.error("- Update at least one documentation file:");
console.error("  - README.md");
console.error("  - RUN_AND_USE.md");
console.error("  - CHANGELOG.md");
console.error("  - docs/*");
process.exit(1);

function readArgValue(flag) {
  const match = args.find((arg) => arg.startsWith(`${flag}=`));
  if (!match) {
    return null;
  }
  const value = match.slice(flag.length + 1).trim();
  return value || null;
}

function resolveBaseRef(explicit) {
  if (explicit) {
    ensureGitRef(explicit);
    return explicit;
  }

  const baseRef = process.env.GITHUB_BASE_REF;
  if (baseRef) {
    const remoteRef = `origin/${baseRef}`;
    ensureGitRef(remoteRef, baseRef);
    return remoteRef;
  }

  const before = (process.env.GITHUB_EVENT_BEFORE ?? "").trim();
  if (before && before !== "0000000000000000000000000000000000000000") {
    ensureGitRef(before);
    return before;
  }

  ensureGitRef("HEAD~1");
  return "HEAD~1";
}

function ensureGitRef(ref, fallbackFetchRef = "") {
  if (git(["rev-parse", "--verify", ref]).status === 0) {
    return;
  }

  if (fallbackFetchRef) {
    git(["fetch", "--no-tags", "--depth=1", "origin", fallbackFetchRef], true);
    if (git(["rev-parse", "--verify", ref]).status === 0) {
      return;
    }
  }

  console.error(`doc-impact-check: unable to resolve git ref: ${ref}`);
  process.exit(1);
}

function listChangedFiles(base, head) {
  let result = git(["diff", "--name-only", `${base}...${head}`]);
  if (result.status !== 0) {
    result = git(["diff", "--name-only", `${base}`, `${head}`]);
  }
  if (result.status !== 0) {
    console.error("doc-impact-check: unable to determine changed files");
    process.exit(1);
  }

  const changed = splitLines(result.stdout);

  // Local developer augmentation: include working tree and untracked changes.
  const staged = git(["diff", "--name-only", "--cached"], true);
  const unstaged = git(["diff", "--name-only"], true);
  const untracked = git(["ls-files", "--others", "--exclude-standard"], true);
  if (staged.status !== 0 || unstaged.status !== 0 || untracked.status !== 0) {
    return changed;
  }

  return Array.from(
    new Set([
      ...changed,
      ...splitLines(staged.stdout),
      ...splitLines(unstaged.stdout),
      ...splitLines(untracked.stdout),
    ]),
  );
}

function isDocsFile(filePath) {
  if (docsFiles.has(filePath)) {
    return true;
  }
  return docsPrefixes.some((prefix) => filePath.startsWith(prefix));
}

function isImpactFile(filePath) {
  if (impactFiles.has(filePath)) {
    return true;
  }
  return impactPrefixes.some((prefix) => filePath.startsWith(prefix));
}

function isNonBehaviorChange(filePath) {
  if (filePath.endsWith("_test.go")) {
    return true;
  }
  if (filePath.includes("/__tests__/")) {
    return true;
  }
  if (filePath.endsWith(".test.ts") || filePath.endsWith(".test.tsx")) {
    return true;
  }
  if (filePath.endsWith(".spec.ts") || filePath.endsWith(".spec.tsx")) {
    return true;
  }
  if (filePath.startsWith("e2e/")) {
    return true;
  }
  return false;
}

function splitLines(value) {
  return (value ?? "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
}

function git(gitArgs, quiet = false) {
  const result = spawnSync("git", gitArgs, {
    cwd: process.cwd(),
    encoding: "utf8",
  });

  if (!quiet && result.status !== 0) {
    const stderr = (result.stderr ?? "").trim();
    if (stderr) {
      console.error(stderr);
    }
  }

  return result;
}
