import { spawnSync } from "node:child_process";
import { accessSync, constants } from "node:fs";
import path from "node:path";

function run(command, args = [], extraEnv = {}) {
  const result = spawnSync(command, args, {
    stdio: "inherit",
    env: {
      ...process.env,
      ...extraEnv,
    },
  });

  if (result.error) {
    console.error(result.error.message);
    process.exit(1);
  }

  if ((result.status ?? 1) !== 0) {
    process.exit(result.status ?? 1);
  }
}

function hasCommand(command) {
  const pathValue = process.env.PATH ?? "";
  const extensions = process.platform === "win32" ? (process.env.PATHEXT ?? ".EXE;.CMD;.BAT").split(";") : [""];
  for (const dir of pathValue.split(path.delimiter)) {
    if (!dir) {
      continue;
    }
    for (const ext of extensions) {
      const candidate = path.join(dir, command + ext);
      try {
        accessSync(candidate, constants.X_OK);
        return true;
      } catch {
        // Keep searching PATH.
      }
    }
  }
  return false;
}

function ensureRaceCompiler() {
  if (hasCommand("gcc") || hasCommand("clang")) {
    return;
  }

  console.error("[ci:backend] Missing C compiler for `go test -race`.");
  console.error("[ci:backend] Install `gcc` or `clang` and make sure it is available in PATH.");
  if (process.platform === "win32") {
    console.error("[ci:backend] Windows tip: install MSYS2 MinGW-w64 and add `<msys2>\\mingw64\\bin` to PATH.");
  } else if (process.platform === "darwin") {
    console.error("[ci:backend] macOS tip: run `xcode-select --install`.");
  } else {
    console.error("[ci:backend] Linux tip: install build-essential (Debian/Ubuntu) or gcc (RHEL/Fedora/Alpine).");
  }
  process.exit(1);
}

run("npm", ["run", "fmt:go"]);
run("git", ["diff", "--exit-code", "--", "backend/cmd", "backend/internal"]);
run("node", ["scripts/go-task.mjs", "-C", "backend", "vet", "./..."]);
run("node", ["scripts/go-task.mjs", "-C", "backend", "run", "github.com/gordonklaus/ineffassign@v0.2.0", "./..."]);
ensureRaceCompiler();
run("node", ["scripts/go-task.mjs", "-C", "backend", "test", "-race", "-timeout", "5m", "./..."], { CGO_ENABLED: "1" });
