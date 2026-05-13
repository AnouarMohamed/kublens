import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import path from "node:path";

const root = process.cwd();
const cacheDir = process.env.GOCACHE || path.join(root, ".gocache");
const modCacheDir = process.env.GOMODCACHE || path.join(root, ".gomodcache");
const tmpDir = process.env.GOTMPDIR || path.join(root, ".tmp-go");
const minimumGoVersion = "1.26.3";
const goToolchain = process.env.GOTOOLCHAIN || `go${minimumGoVersion}+auto`;

for (const dir of [cacheDir, modCacheDir, tmpDir]) {
  mkdirSync(dir, { recursive: true });
}

const args = process.argv.slice(2);
const result = spawnSync("go", args, {
  stdio: "inherit",
  env: {
    ...process.env,
    GOCACHE: cacheDir,
    GOMODCACHE: modCacheDir,
    GOTMPDIR: tmpDir,
    GOTOOLCHAIN: goToolchain,
  },
});

if (result.error) {
  if (result.error.code === "ENOENT") {
    const attemptedCommand = ["go", ...args].join(" ");
    console.error("Go toolchain not found on PATH.");
    console.error(
      `Install Go ${minimumGoVersion}+ to run backend commands such as npm run dev:api, npm run test:go, and npm run ci:backend.`,
    );
    console.error(`Attempted command: ${attemptedCommand}`);
    process.exit(1);
  }
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status ?? 1);
