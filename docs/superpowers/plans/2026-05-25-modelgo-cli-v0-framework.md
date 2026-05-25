# modelgo-cli v0 Framework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the minimum `modelgo-cli` framework so `npx @modelgo/cli@latest install` produces (a) a global `modelgo-cli` binary, (b) SKILL.md files distributed to every supported AI agent, and (c) one end-to-end demo (`modelgo-cli hello`) that an AI agent can call.

**Architecture:** Single-repo monorepo at `github.com/modelgo/modelgo-cli`. A Go binary (cross-compiled to 6 platforms via GoReleaser, hosted on GitHub Releases) is wrapped by an npm package `@modelgo/cli` whose `postinstall` downloads the right binary and whose `install` subcommand runs an interactive wizard. Skills live in `skills/` and are distributed via `npx skills add modelgo/modelgo-cli`.

**Tech Stack:** Go 1.22 (binary), Node.js 20+ (npm wrapper + tests), `@clack/prompts` (wizard UI), GoReleaser (release pipeline), GitHub Actions (CI + release), `node:test` (npm wrapper unit tests).

**Spec:** `docs/superpowers/specs/2026-05-25-modelgo-cli-design.md`

**Pre-flight (NOT in plan, must be done by humans first):**
- Confirm npm scope `@modelgo` is available or already owned by modelgo
- Create npm automation token, add as `NPM_TOKEN` secret in `github.com/modelgo/modelgo-cli`
- Verify GitHub Actions is enabled on the repo with `contents: write` permission

---

### Task 1: Project bootstrap files

**Files:**
- Create: `~/code/modelgo/modelgo-cli/.gitignore`
- Create: `~/code/modelgo/modelgo-cli/LICENSE`
- Modify: `~/code/modelgo/modelgo-cli/README.md`

- [ ] **Step 1: Write `.gitignore`**

```
# Go build output (downloaded by postinstall at user install time)
bin/
*.exe

# GoReleaser output
dist/

# Node
node_modules/
npm-debug.log*

# OS
.DS_Store
Thumbs.db
```

- [ ] **Step 2: Write `LICENSE` (MIT)**

```
MIT License

Copyright (c) 2026 modelgo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 3: Write a placeholder `README.md`** (will be expanded in Task 17)

```markdown
# modelgo-cli

The official CLI for modelgo. Pairs with AI agent skills (Claude Code, Codex, Gemini CLI, etc.) so AI agents can operate modelgo on your behalf.

> Status: v0 framework — no business APIs wired up yet.

## Install

```bash
npx @modelgo/cli@latest install
```

See `docs/superpowers/specs/2026-05-25-modelgo-cli-design.md` for the v0 design.
```

- [ ] **Step 4: Commit**

```bash
cd ~/code/modelgo/modelgo-cli
git add .gitignore LICENSE README.md
git commit -m "chore: bootstrap project files (gitignore, MIT license, README stub)"
```

---

### Task 2: Initialize Go module

**Files:**
- Create: `~/code/modelgo/modelgo-cli/go.mod`

- [ ] **Step 1: Run `go mod init`**

```bash
cd ~/code/modelgo/modelgo-cli
go mod init github.com/modelgo/modelgo-cli
```

Expected output: `go: creating new go.mod: module github.com/modelgo/modelgo-cli`

- [ ] **Step 2: Verify `go.mod` content**

```bash
cat go.mod
```

Expected:
```
module github.com/modelgo/modelgo-cli

go 1.22
```

If `go` directive shows a different version (depends on local Go install), that's fine — leave it.

- [ ] **Step 3: Commit**

```bash
git add go.mod
git commit -m "chore(go): initialize go module github.com/modelgo/modelgo-cli"
```

---

### Task 3: `internal/version` package (TDD)

**Files:**
- Create: `~/code/modelgo/modelgo-cli/internal/version/version.go`
- Create: `~/code/modelgo/modelgo-cli/internal/version/version_test.go`

- [ ] **Step 1: Write the failing test**

`internal/version/version_test.go`:

```go
package version

import "testing"

func TestDefaultIsDev(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("expected default Version to be %q, got %q", "dev", Version)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd ~/code/modelgo/modelgo-cli
go test ./internal/version/...
```

Expected: build error like `undefined: Version`. That's the failing test for TDD.

- [ ] **Step 3: Write minimal implementation**

`internal/version/version.go`:

```go
// Package version exposes the modelgo-cli version string.
// Set at build time via -ldflags "-X github.com/modelgo/modelgo-cli/internal/version.Version=v1.2.3".
package version

var Version = "dev"
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/version/...
```

Expected: `ok  github.com/modelgo/modelgo-cli/internal/version  ...`

- [ ] **Step 5: Verify ldflags injection works**

```bash
cd ~/code/modelgo/modelgo-cli
go test -ldflags "-X github.com/modelgo/modelgo-cli/internal/version.Version=v0.1.0" ./internal/version/...
```

Expected: FAIL — confirms the test catches the override (which means injection works).

- [ ] **Step 6: Commit**

```bash
git add internal/version/
git commit -m "feat(version): add version package with build-time ldflags injection"
```

---

### Task 4: `internal/hello` package (TDD)

**Files:**
- Create: `~/code/modelgo/modelgo-cli/internal/hello/hello.go`
- Create: `~/code/modelgo/modelgo-cli/internal/hello/hello_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/hello/hello_test.go`:

```go
package hello

import "testing"

func TestGreetDefault(t *testing.T) {
	got := Greet("")
	want := "Hello, world!"
	if got != want {
		t.Errorf("Greet(\"\") = %q, want %q", got, want)
	}
}

func TestGreetWithName(t *testing.T) {
	got := Greet("渭哲")
	want := "Hello, 渭哲!"
	if got != want {
		t.Errorf("Greet(\"渭哲\") = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/hello/...
```

Expected: build error `undefined: Greet`.

- [ ] **Step 3: Write minimal implementation**

`internal/hello/hello.go`:

```go
// Package hello implements the demo `modelgo-cli hello` subcommand.
package hello

import "fmt"

// Greet returns "Hello, <name>!", defaulting to "world" when name is empty.
func Greet(name string) string {
	if name == "" {
		name = "world"
	}
	return fmt.Sprintf("Hello, %s!", name)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/hello/...
```

Expected: `ok  github.com/modelgo/modelgo-cli/internal/hello  ...`

- [ ] **Step 5: Commit**

```bash
git add internal/hello/
git commit -m "feat(hello): add Greet() with default and named cases"
```

---

### Task 5: `cmd/modelgo-cli/main.go` + binary build smoke test

**Files:**
- Create: `~/code/modelgo/modelgo-cli/cmd/modelgo-cli/main.go`

- [ ] **Step 1: Write `main.go`**

```go
// Command modelgo-cli is the modelgo CLI entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/modelgo/modelgo-cli/internal/hello"
	"github.com/modelgo/modelgo-cli/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "--version", "-v":
		fmt.Println(version.Version)
	case "--help", "-h":
		printUsage(os.Stdout)
	case "hello":
		runHello(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func runHello(args []string) {
	fs := flag.NewFlagSet("hello", flag.ExitOnError)
	name := fs.String("name", "world", "name to greet")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	fmt.Println(hello.Greet(*name))
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `modelgo-cli — the official modelgo CLI

USAGE:
    modelgo-cli <command> [flags]

COMMANDS:
    hello [--name NAME]   Print a greeting
    --version, -v         Print the version
    --help, -h            Show this help`)
}
```

- [ ] **Step 2: Build the binary**

```bash
cd ~/code/modelgo/modelgo-cli
go build -o /tmp/modelgo-cli ./cmd/modelgo-cli
```

Expected: no output, exit 0.

- [ ] **Step 3: Smoke test `--version` with ldflags injection**

```bash
go build -ldflags "-X github.com/modelgo/modelgo-cli/internal/version.Version=v0.1.0-test" \
    -o /tmp/modelgo-cli ./cmd/modelgo-cli
/tmp/modelgo-cli --version
```

Expected output: `v0.1.0-test`

- [ ] **Step 4: Smoke test `hello`**

```bash
/tmp/modelgo-cli hello
/tmp/modelgo-cli hello --name 渭哲
```

Expected:
```
Hello, world!
Hello, 渭哲!
```

- [ ] **Step 5: Smoke test unknown command exits 2**

```bash
/tmp/modelgo-cli wat; echo "exit=$?"
```

Expected: usage on stderr + `exit=2`.

- [ ] **Step 6: Commit**

```bash
rm -f /tmp/modelgo-cli
git add cmd/modelgo-cli/
git commit -m "feat(cli): add main entrypoint with --version, --help, hello subcommands"
```

---

### Task 6: `package.json`

**Files:**
- Create: `~/code/modelgo/modelgo-cli/package.json`

- [ ] **Step 1: Write `package.json`**

```json
{
  "name": "@modelgo/cli",
  "version": "0.1.0",
  "description": "The official CLI for modelgo. Pairs with AI agent skills so AI agents can operate modelgo on your behalf.",
  "bin": {
    "modelgo-cli": "scripts/run.js"
  },
  "scripts": {
    "postinstall": "node scripts/install.js",
    "test": "node --test scripts/__tests__/*.test.mjs",
    "lint:skills": "node scripts/lint-skills.mjs"
  },
  "files": [
    "scripts/install.js",
    "scripts/install-wizard.js",
    "scripts/run.js",
    "checksums.txt",
    "LICENSE",
    "README.md"
  ],
  "os": ["darwin", "linux", "win32"],
  "cpu": ["x64", "arm64"],
  "engines": {
    "node": ">=16"
  },
  "dependencies": {
    "@clack/prompts": "^1.2.0"
  },
  "repository": {
    "type": "git",
    "url": "git+https://github.com/modelgo/modelgo-cli.git"
  },
  "homepage": "https://github.com/modelgo/modelgo-cli#readme",
  "bugs": {
    "url": "https://github.com/modelgo/modelgo-cli/issues"
  },
  "license": "MIT",
  "publishConfig": {
    "access": "public"
  }
}
```

- [ ] **Step 2: Install deps locally so tests can run**

```bash
cd ~/code/modelgo/modelgo-cli
npm install --ignore-scripts
```

The `--ignore-scripts` flag prevents the postinstall (which doesn't exist yet) from running.

Expected: `node_modules/` appears, `package-lock.json` is created.

- [ ] **Step 3: Verify `@clack/prompts` is installed**

```bash
ls node_modules/@clack/prompts/package.json
```

Expected: file exists.

- [ ] **Step 4: Commit**

```bash
git add package.json package-lock.json
git commit -m "chore(npm): add package.json manifest for @modelgo/cli"
```

---

### Task 7: `scripts/run.js` + manual verification

**Files:**
- Create: `~/code/modelgo/modelgo-cli/scripts/run.js`

- [ ] **Step 1: Write `scripts/run.js`**

```js
#!/usr/bin/env node
// Copyright (c) 2026 modelgo
// SPDX-License-Identifier: MIT
//
// npm bin entrypoint. Routes:
//   - `modelgo-cli install`  → install-wizard.js (interactive setup)
//   - everything else        → exec bin/modelgo-cli (the downloaded Go binary)

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const args = process.argv.slice(2);

if (args[0] === "install") {
  require("./install-wizard.js");
} else {
  const isWindows = process.platform === "win32";
  const binName = "modelgo-cli" + (isWindows ? ".exe" : "");
  const binPath = path.join(__dirname, "..", "bin", binName);

  if (!fs.existsSync(binPath)) {
    console.error(
      `modelgo-cli binary not found at ${binPath}\n` +
      `Please reinstall: npm install -g @modelgo/cli`
    );
    process.exit(1);
  }

  try {
    execFileSync(binPath, args, { stdio: "inherit" });
  } catch (e) {
    process.exit(typeof e.status === "number" ? e.status : 1);
  }
}
```

- [ ] **Step 2: Manual verification — missing-binary path**

```bash
cd ~/code/modelgo/modelgo-cli
node scripts/run.js --version; echo "exit=$?"
```

Expected: stderr says binary not found at `.../bin/modelgo-cli`, `exit=1`.

- [ ] **Step 3: Manual verification — binary present path**

```bash
mkdir -p bin
go build -ldflags "-X github.com/modelgo/modelgo-cli/internal/version.Version=v0.0.0-dev" \
    -o bin/modelgo-cli ./cmd/modelgo-cli
node scripts/run.js --version
node scripts/run.js hello --name 渭哲
rm -rf bin
```

Expected:
```
v0.0.0-dev
Hello, 渭哲!
```

- [ ] **Step 4: Commit**

```bash
git add scripts/run.js
git commit -m "feat(npm): add scripts/run.js bin entrypoint with install routing"
```

---

### Task 8: `scripts/install.js` — pure helpers (TDD)

This task writes the pure utility functions in `install.js` and tests them. The download / extract / install() orchestration comes in Task 9.

**Files:**
- Create: `~/code/modelgo/modelgo-cli/scripts/install.js` (partial — pure functions only this task)
- Create: `~/code/modelgo/modelgo-cli/scripts/__tests__/install.test.mjs`
- Create: `~/code/modelgo/modelgo-cli/scripts/__tests__/fixtures/checksums.txt`

- [ ] **Step 1: Write the failing tests**

`scripts/__tests__/install.test.mjs`:

```js
// Tests for pure helpers in scripts/install.js
// Run: npm test

import { test } from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import os from "node:os";
import fs from "node:fs";
import { fileURLToPath } from "node:url";

import {
  resolveMirrorUrls,
  isValidDownloadBase,
  isDefaultNpmjsRegistry,
  assertAllowedHost,
  ALLOWED_HOSTS,
  getExpectedChecksum,
  verifyChecksum,
  semverLessThan,
} from "../install.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const fixturesDir = path.join(__dirname, "fixtures");

test("resolveMirrorUrls: default chain when no registry set", () => {
  const urls = resolveMirrorUrls({}, "modelgo-cli-0.1.0-darwin-arm64.tar.gz", "0.1.0");
  assert.deepEqual(urls, [
    "https://registry.npmmirror.com/-/binary/modelgo-cli/v0.1.0/modelgo-cli-0.1.0-darwin-arm64.tar.gz",
  ]);
});

test("resolveMirrorUrls: custom https registry prepended, default appended", () => {
  const urls = resolveMirrorUrls(
    { npm_config_registry: "https://corp.example.com/npm/" },
    "modelgo-cli-0.1.0-linux-amd64.tar.gz",
    "0.1.0"
  );
  assert.deepEqual(urls, [
    "https://corp.example.com/npm/-/binary/modelgo-cli/v0.1.0/modelgo-cli-0.1.0-linux-amd64.tar.gz",
    "https://registry.npmmirror.com/-/binary/modelgo-cli/v0.1.0/modelgo-cli-0.1.0-linux-amd64.tar.gz",
  ]);
});

test("resolveMirrorUrls: http registry rejected (only default returned)", () => {
  const urls = resolveMirrorUrls(
    { npm_config_registry: "http://insecure.example.com/" },
    "modelgo-cli-0.1.0-linux-amd64.tar.gz",
    "0.1.0"
  );
  assert.equal(urls.length, 1);
  assert.equal(urls[0], "https://registry.npmmirror.com/-/binary/modelgo-cli/v0.1.0/modelgo-cli-0.1.0-linux-amd64.tar.gz");
});

test("resolveMirrorUrls: default npmjs registry skipped (only default returned)", () => {
  const urls = resolveMirrorUrls(
    { npm_config_registry: "https://registry.npmjs.org/" },
    "modelgo-cli-0.1.0-linux-amd64.tar.gz",
    "0.1.0"
  );
  assert.equal(urls.length, 1);
});

test("isValidDownloadBase: rejects malformed and non-https", () => {
  assert.equal(isValidDownloadBase("not a url"), false);
  assert.equal(isValidDownloadBase("ftp://example.com"), false);
  assert.equal(isValidDownloadBase("http://example.com"), false);
  assert.equal(isValidDownloadBase("https://example.com"), true);
});

test("isDefaultNpmjsRegistry: matches host only", () => {
  assert.equal(isDefaultNpmjsRegistry("https://registry.npmjs.org/"), true);
  assert.equal(isDefaultNpmjsRegistry("https://registry.npmjs.org"), true);
  assert.equal(isDefaultNpmjsRegistry("https://registry.npmmirror.com/"), false);
  assert.equal(isDefaultNpmjsRegistry("not-a-url"), false);
});

test("assertAllowedHost: passes for allowed, throws for others", () => {
  assert.doesNotThrow(() => assertAllowedHost("https://github.com/foo"));
  assert.doesNotThrow(() => assertAllowedHost("https://registry.npmmirror.com/foo"));
  assert.throws(() => assertAllowedHost("https://evil.example.com/foo"), /Download host not allowed/);
});

test("getExpectedChecksum: finds entry in fixtures/checksums.txt", () => {
  const hash = getExpectedChecksum("modelgo-cli-0.1.0-darwin-arm64.tar.gz", fixturesDir);
  assert.equal(hash, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
});

test("getExpectedChecksum: throws when archive entry missing", () => {
  assert.throws(
    () => getExpectedChecksum("does-not-exist.tar.gz", fixturesDir),
    /Checksum entry not found/
  );
});

test("verifyChecksum: passes for matching SHA-256", () => {
  // Write 5 bytes, compute SHA-256 deterministically
  const tmp = path.join(os.tmpdir(), "modelgo-cli-test-" + Date.now());
  fs.writeFileSync(tmp, "hello");
  // SHA-256 of "hello" = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
  try {
    assert.doesNotThrow(() =>
      verifyChecksum(tmp, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
    );
  } finally {
    fs.unlinkSync(tmp);
  }
});

test("verifyChecksum: throws for mismatch", () => {
  const tmp = path.join(os.tmpdir(), "modelgo-cli-test-" + Date.now());
  fs.writeFileSync(tmp, "hello");
  try {
    assert.throws(
      () => verifyChecksum(tmp, "0".repeat(64)),
      /SECURITY.*Checksum mismatch/
    );
  } finally {
    fs.unlinkSync(tmp);
  }
});

test("verifyChecksum: null hash skips verification (warning path)", () => {
  // expectedHash === null means checksums.txt absent — fall through silently
  assert.doesNotThrow(() => verifyChecksum("/nonexistent", null));
});

test("semverLessThan: numeric comparison", () => {
  assert.equal(semverLessThan("1.0.0", "1.0.1"), true);
  assert.equal(semverLessThan("1.0.1", "1.0.0"), false);
  assert.equal(semverLessThan("1.0.0", "1.0.0"), false);
  assert.equal(semverLessThan("1.10.0", "1.9.0"), false);
  assert.equal(semverLessThan("2.0.0-beta", "2.0.0"), false); // pre-release suffix stripped
});
```

- [ ] **Step 2: Write the fixture file**

`scripts/__tests__/fixtures/checksums.txt`:

```
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  modelgo-cli-0.1.0-darwin-arm64.tar.gz
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  modelgo-cli-0.1.0-linux-amd64.tar.gz
```

(Note: two spaces between hash and filename — matches GoReleaser output format.)

- [ ] **Step 3: Run test to verify it fails**

```bash
cd ~/code/modelgo/modelgo-cli
npm test
```

Expected: tests fail because `scripts/install.js` doesn't exist yet.

- [ ] **Step 4: Write `scripts/install.js` with pure helpers + module exports**

```js
// Copyright (c) 2026 modelgo
// SPDX-License-Identifier: MIT

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");
const os = require("os");
const crypto = require("crypto");

const VERSION = require("../package.json").version.replace(/-.*$/, "");
const REPO = "modelgo/modelgo-cli";
const NAME = "modelgo-cli";
const DEFAULT_MIRROR_HOST = "https://registry.npmmirror.com";

// Allowlist gates the initial request host. curl --location follows redirects
// up to --max-redirs without re-checking; checksum verification is the
// primary integrity control, this allowlist is defense-in-depth.
const ALLOWED_HOSTS = new Set([
  "github.com",
  "objects.githubusercontent.com",
  "registry.npmmirror.com",
]);

const PLATFORM_MAP = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };

const platform = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];
const isWindows = process.platform === "win32";
const ext = isWindows ? ".zip" : ".tar.gz";
const archiveName = `${NAME}-${VERSION}-${platform}-${arch}${ext}`;
const GITHUB_URL = `https://github.com/${REPO}/releases/download/v${VERSION}/${archiveName}`;

const binDir = path.join(__dirname, "..", "bin");
const dest = path.join(binDir, NAME + (isWindows ? ".exe" : ""));

function joinUrl(base, suffix) {
  return base.replace(/\/+$/, "") + suffix;
}

function isValidDownloadBase(raw) {
  try {
    const parsed = new URL(raw);
    return parsed.protocol === "https:" && !!parsed.hostname;
  } catch (_) {
    return false;
  }
}

function isDefaultNpmjsRegistry(url) {
  try {
    const { hostname } = new URL(url);
    return hostname === "registry.npmjs.org";
  } catch (_) {
    return false;
  }
}

function assertAllowedHost(url) {
  const { hostname } = new URL(url);
  if (!ALLOWED_HOSTS.has(hostname)) {
    throw new Error(`Download host not allowed: ${hostname}`);
  }
}

function resolveMirrorUrls(env, archive, version) {
  const binaryPath = `/-/binary/${NAME}/v${version}/${archive}`;
  const defaultUrl = joinUrl(DEFAULT_MIRROR_HOST, binaryPath);

  const urls = [];
  const registry = (env.npm_config_registry || "").trim();
  if (registry && !isDefaultNpmjsRegistry(registry) && isValidDownloadBase(registry)) {
    const base = new URL(registry);
    urls.push(joinUrl(base.origin + base.pathname, binaryPath));
  }
  if (!urls.includes(defaultUrl)) urls.push(defaultUrl);
  return urls;
}

function getExpectedChecksum(archiveName, checksumsDir) {
  const dir = checksumsDir || path.join(__dirname, "..");
  const checksumsPath = path.join(dir, "checksums.txt");

  if (!fs.existsSync(checksumsPath)) {
    console.error("[WARN] checksums.txt not found, skipping checksum verification");
    return null;
  }

  const content = fs.readFileSync(checksumsPath, "utf8");
  for (const line of content.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const idx = trimmed.indexOf("  ");
    if (idx === -1) continue;
    const hash = trimmed.slice(0, idx);
    const name = trimmed.slice(idx + 2);
    if (name === archiveName) return hash;
  }

  throw new Error(`Checksum entry not found for ${archiveName}`);
}

function verifyChecksum(archivePath, expectedHash) {
  if (expectedHash === null) return;

  const hash = crypto.createHash("sha256");
  const fd = fs.openSync(archivePath, "r");
  try {
    const buf = Buffer.alloc(64 * 1024);
    let bytesRead;
    while ((bytesRead = fs.readSync(fd, buf, 0, buf.length, null)) > 0) {
      hash.update(buf.subarray(0, bytesRead));
    }
  } finally {
    fs.closeSync(fd);
  }
  const actual = hash.digest("hex");

  if (actual.toLowerCase() !== expectedHash.toLowerCase()) {
    throw new Error(
      `[SECURITY] Checksum mismatch for ${path.basename(archivePath)}: ` +
      `expected ${expectedHash} but got ${actual}`
    );
  }
}

function semverLessThan(a, b) {
  const pa = a.replace(/-.*$/, "").split(".").map(Number);
  const pb = b.replace(/-.*$/, "").split(".").map(Number);
  for (let i = 0; i < 3; i++) {
    if ((pa[i] || 0) < (pb[i] || 0)) return true;
    if ((pa[i] || 0) > (pb[i] || 0)) return false;
  }
  return false;
}

// Note: download/extract/install() are added in Task 9.

module.exports = {
  // exported for testing
  ALLOWED_HOSTS,
  resolveMirrorUrls,
  isValidDownloadBase,
  isDefaultNpmjsRegistry,
  assertAllowedHost,
  getExpectedChecksum,
  verifyChecksum,
  semverLessThan,
  // exported for install-wizard
  archiveName,
  VERSION,
  binDir,
  dest,
};
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
npm test
```

Expected: all tests pass (12 tests).

- [ ] **Step 6: Commit**

```bash
git add scripts/install.js scripts/__tests__/install.test.mjs scripts/__tests__/fixtures/checksums.txt
git commit -m "feat(npm): add install.js pure helpers (mirror URL, checksum, semver) with tests"
```

---

### Task 9: `scripts/install.js` — download + extract + install orchestration

**Files:**
- Modify: `~/code/modelgo/modelgo-cli/scripts/install.js` (append the orchestration code)

- [ ] **Step 1: Append download/extract/install/main block to `install.js`**

Replace the `// Note: download/extract/install() are added in Task 9.` line with:

```js
function getMirrorUrls(env) {
  const urls = resolveMirrorUrls(env, archiveName, VERSION);
  for (const u of urls) ALLOWED_HOSTS.add(new URL(u).hostname);
  return urls;
}

function download(url, destPath) {
  assertAllowedHost(url);
  const args = [
    "--fail", "--location", "--silent", "--show-error",
    "--connect-timeout", "10", "--max-time", "120",
    "--max-redirs", "3",
    "--output", destPath,
  ];
  if (isWindows) args.unshift("--ssl-revoke-best-effort");
  args.push(url);
  execFileSync("curl", args, { stdio: ["ignore", "ignore", "pipe"] });
}

function extractZipWindows(archivePath, destDir) {
  const psOpts = ["-NoProfile", "-ExecutionPolicy", "Bypass", "-Command"];
  const psStdio = ["ignore", "inherit", "inherit"];
  const psEnv = {
    ...process.env,
    MODELGO_CLI_ARCHIVE: archivePath,
    MODELGO_CLI_DEST: destDir,
  };
  try {
    const dotnet =
      "$ErrorActionPreference='Stop';" +
      "Add-Type -AssemblyName System.IO.Compression.FileSystem;" +
      "[System.IO.Compression.ZipFile]::ExtractToDirectory($env:MODELGO_CLI_ARCHIVE,$env:MODELGO_CLI_DEST)";
    execFileSync("powershell.exe", [...psOpts, dotnet], { stdio: psStdio, env: psEnv });
  } catch (primaryErr) {
    try {
      const cmdlet =
        "$ErrorActionPreference='Stop';" +
        "Expand-Archive -LiteralPath $env:MODELGO_CLI_ARCHIVE -DestinationPath $env:MODELGO_CLI_DEST -Force";
      execFileSync("powershell.exe", [...psOpts, cmdlet], { stdio: psStdio, env: psEnv });
    } catch (secondErr) {
      try {
        execFileSync("tar", ["-xf", archivePath, "-C", destDir], { stdio: psStdio });
      } catch (fallbackErr) {
        throw new Error(
          `Failed to extract ${archivePath}. ` +
          `.NET ZipFile: ${primaryErr.message}. ` +
          `Expand-Archive: ${secondErr.message}. ` +
          `tar: ${fallbackErr.message}`
        );
      }
    }
  }
}

function install() {
  const mirrorUrls = getMirrorUrls(process.env);
  const downloadUrls = [GITHUB_URL, ...mirrorUrls];

  fs.mkdirSync(binDir, { recursive: true });
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "modelgo-cli-"));
  const archivePath = path.join(tmpDir, archiveName);

  try {
    let lastErr;
    let downloaded = false;
    for (const url of downloadUrls) {
      try {
        download(url, archivePath);
        downloaded = true;
        break;
      } catch (e) {
        lastErr = e;
      }
    }
    if (!downloaded) throw lastErr;

    const expectedHash = getExpectedChecksum(archiveName);
    verifyChecksum(archivePath, expectedHash);

    if (isWindows) {
      extractZipWindows(archivePath, tmpDir);
    } else {
      execFileSync("tar", ["-xzf", archivePath, "-C", tmpDir], { stdio: "ignore" });
    }

    const binaryName = NAME + (isWindows ? ".exe" : "");
    const extractedBinary = path.join(tmpDir, binaryName);
    fs.copyFileSync(extractedBinary, dest);
    fs.chmodSync(dest, 0o755);
    console.log(`${NAME} v${VERSION} installed successfully`);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

if (require.main === module) {
  if (!platform || !arch) {
    console.error(`Unsupported platform: ${process.platform}-${process.arch}`);
    process.exit(1);
  }

  // Skip binary download when triggered as postinstall under `npx <pkg> install`.
  // The wizard does not need the binary; run.js will be called next, and it
  // dispatches to install-wizard.js for the "install" arg.
  const isNpxPostinstall = process.env.npm_command === "exec";
  if (isNpxPostinstall) {
    process.exit(0);
  }

  try {
    install();
  } catch (err) {
    console.error(`Failed to install ${NAME}:`, err.message);
    console.error(
      `\nIf you are behind a firewall or in a restricted network, try one of:\n` +
      `  # 1. Use a proxy:\n` +
      `  export https_proxy=http://your-proxy:port\n` +
      `  npm install -g @modelgo/cli\n\n` +
      `  # 2. Point to a corporate npm mirror that proxies /-/binary/${NAME}/...:\n` +
      `  npm install -g @modelgo/cli --registry=https://your-corp-mirror/`
    );
    process.exit(1);
  }
}
```

- [ ] **Step 2: Re-run tests to confirm pure helpers still pass**

```bash
npm test
```

Expected: all tests still pass.

- [ ] **Step 3: Manual smoke test — npx-postinstall path exits cleanly**

```bash
cd ~/code/modelgo/modelgo-cli
npm_command=exec node scripts/install.js; echo "exit=$?"
```

Expected: `exit=0`, no output (the script saw `npm_command=exec` and exited).

- [ ] **Step 4: Manual smoke test — postinstall path fails gracefully when binary doesn't exist on GitHub yet**

```bash
node scripts/install.js; echo "exit=$?"
```

Expected: `exit=1`, error message about download failure (since we haven't tagged a release). This is the expected behavior — confirms error handling works.

- [ ] **Step 5: Commit**

```bash
git add scripts/install.js
git commit -m "feat(npm): add install.js download/extract/install orchestration"
```

---

### Task 10: `scripts/install-wizard.js`

**Files:**
- Create: `~/code/modelgo/modelgo-cli/scripts/install-wizard.js`

- [ ] **Step 1: Write `scripts/install-wizard.js`**

```js
#!/usr/bin/env node
// Copyright (c) 2026 modelgo
// SPDX-License-Identifier: MIT

const fs = require("fs");
const path = require("path");
const { execFileSync, execFile } = require("child_process");
const p = require("@clack/prompts");

const PKG = "@modelgo/cli";
const SKILLS_REPO = "modelgo/modelgo-cli";
const isWindows = process.platform === "win32";

// ---------------------------------------------------------------------------
// i18n
// ---------------------------------------------------------------------------

const messages = {
  zh: {
    setup:         "正在设置 modelgo CLI...",
    step1:         "正在安装 %s...",
    step1Upgrade:  "正在升级 %s (v%s → v%s)...",
    step1Skip:     "已安装 (v%s)，跳过",
    step1Done:     "已全局安装",
    step1Upgraded: "已升级到 v%s",
    step1Fail:     "全局安装失败。运行以下命令重试: npm install -g %s",
    step2Spinner:  "正在安装 Skills...",
    step2Skip:     "Skills 已安装，跳过",
    step2Done:     "Skills 已安装",
    step2Fail:     "Skills 安装失败。运行以下命令重试: npx skills add %s -y -g",
    done:          "安装完成！\n试试跟你的 AI 工具（Claude Code、Codex 等）说：\"用 modelgo-cli 跟我打个招呼\"",
    cancelled:     "安装已取消",
    nonTtyHint:    "非交互模式，已完成 npm 全局安装 + skills 安装。",
  },
  en: {
    setup:         "Setting up modelgo CLI...",
    step1:         "Installing %s globally...",
    step1Upgrade:  "Upgrading %s (v%s → v%s)...",
    step1Skip:     "Already installed (v%s). Skipped",
    step1Done:     "Installed globally",
    step1Upgraded: "Upgraded to v%s",
    step1Fail:     "Failed to install globally. Run manually: npm install -g %s",
    step2Spinner:  "Installing skills...",
    step2Skip:     "Skills already installed. Skipped",
    step2Done:     "Skills installed",
    step2Fail:     "Failed to install skills. Run manually: npx skills add %s -y -g",
    done:          "You are all set!\nTry asking your AI tool (Claude Code, Codex, etc.): \"Have modelgo-cli say hello to me\"",
    cancelled:     "Installation cancelled",
    nonTtyHint:    "Non-interactive mode. Completed global install + skills install.",
  },
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function execCmd(cmd, args, opts) {
  if (isWindows) {
    return execFileSync("cmd.exe", ["/c", cmd, ...args], opts);
  }
  return execFileSync(cmd, args, opts);
}

function runSilent(cmd, args, opts = {}) {
  return execCmd(cmd, args, { stdio: ["ignore", "pipe", "pipe"], ...opts });
}

function runSilentAsync(cmd, args, opts = {}) {
  const actualCmd = isWindows ? "cmd.exe" : cmd;
  const actualArgs = isWindows ? ["/c", cmd, ...args] : args;
  return new Promise((resolve, reject) => {
    execFile(actualCmd, actualArgs, { stdio: ["ignore", "pipe", "pipe"], ...opts }, (err, stdout) => {
      if (err) reject(err);
      else resolve(stdout);
    });
  });
}

function fmt(template, ...values) {
  let i = 0;
  return template.replace(/%s/g, () => values[i++] ?? "");
}

function getLatestVersion() {
  try {
    const out = runSilent("npm", ["view", PKG, "version"], { timeout: 15000 });
    const ver = out.toString().trim();
    return /^\d+\.\d+\.\d+/.test(ver) ? ver : null;
  } catch (_) {
    return null;
  }
}

function semverLessThan(a, b) {
  const pa = a.replace(/-.*$/, "").split(".").map(Number);
  const pb = b.replace(/-.*$/, "").split(".").map(Number);
  for (let i = 0; i < 3; i++) {
    if ((pa[i] || 0) < (pb[i] || 0)) return true;
    if ((pa[i] || 0) > (pb[i] || 0)) return false;
  }
  return false;
}

function getGloballyInstalledVersion() {
  try {
    const out = runSilent("npm", ["list", "-g", PKG], { timeout: 15000 });
    const match = out.toString().match(/@(\d+\.\d+\.\d+[^\s]*)/);
    return match ? match[1] : "unknown";
  } catch (_) {
    return null;
  }
}

function parseLangArg() {
  const args = process.argv.slice(2);
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--lang" && args[i + 1]) {
      const val = args[i + 1].toLowerCase();
      if (val === "zh" || val === "en") return val;
    }
    if (args[i].startsWith("--lang=")) {
      const val = args[i].split("=")[1].toLowerCase();
      if (val === "zh" || val === "en") return val;
    }
  }
  return null;
}

function handleCancel(value, msg) {
  if (p.isCancel(value)) {
    p.cancel(msg.cancelled);
    process.exit(0);
  }
  return value;
}

// ---------------------------------------------------------------------------
// Steps
// ---------------------------------------------------------------------------

async function stepSelectLang() {
  const fromArg = parseLangArg();
  if (fromArg) return fromArg;
  const lang = await p.select({
    message: "请选择语言 / Select language",
    options: [
      { value: "zh", label: "中文" },
      { value: "en", label: "English" },
    ],
  });
  return handleCancel(lang, messages.zh);
}

async function stepInstallGlobally(msg) {
  const installedVer = getGloballyInstalledVersion();
  const latestVer = getLatestVersion();
  const needsUpgrade = installedVer && latestVer && semverLessThan(installedVer, latestVer);

  if (installedVer && !needsUpgrade) {
    p.log.info(fmt(msg.step1Skip, installedVer));
    return;
  }

  const s = p.spinner();
  if (needsUpgrade) {
    s.start(fmt(msg.step1Upgrade, PKG, installedVer, latestVer));
  } else {
    s.start(fmt(msg.step1, PKG));
  }
  try {
    await runSilentAsync("npm", ["install", "-g", PKG], { timeout: 120000 });
    s.stop(needsUpgrade ? fmt(msg.step1Upgraded, latestVer) : msg.step1Done);
  } catch (_) {
    s.stop(fmt(msg.step1Fail, PKG));
    process.exit(1);
  }
}

async function skillsAlreadyInstalled() {
  try {
    const out = await runSilentAsync("npx", ["-y", "skills", "ls", "-g"], { timeout: 120000 });
    return /^modelgo-/m.test(out.toString());
  } catch (_) {
    return false;
  }
}

async function stepInstallSkills(msg) {
  const s = p.spinner();
  s.start(msg.step2Spinner);
  try {
    if (await skillsAlreadyInstalled()) {
      s.stop(msg.step2Skip);
      return;
    }
    await runSilentAsync("npx", ["-y", "skills", "add", SKILLS_REPO, "-y", "-g"], { timeout: 120000 });
    s.stop(msg.step2Done);
  } catch (_) {
    s.stop(fmt(msg.step2Fail, SKILLS_REPO));
    process.exit(1);
  }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main() {
  const isInteractive = !!process.stdin.isTTY;
  const lang = isInteractive ? await stepSelectLang() : (parseLangArg() || "en");
  const msg = messages[lang];

  if (isInteractive) {
    p.intro(msg.setup);
    await stepInstallGlobally(msg);
    await stepInstallSkills(msg);
    p.outro(msg.done);
  } else {
    console.log(msg.setup);
    await stepInstallGlobally(msg);
    await stepInstallSkills(msg);
    console.log(msg.nonTtyHint);
  }
}

main().catch((err) => {
  p.cancel("Unexpected error: " + (err.message || err));
  process.exit(1);
});
```

- [ ] **Step 2: Manual smoke test — non-interactive mode prints language hint**

```bash
cd ~/code/modelgo/modelgo-cli
echo "" | timeout 5 node scripts/install-wizard.js --lang=en 2>&1 | head -5
```

Expected: prints "Setting up modelgo CLI...". The command will then attempt `npm install -g`, which likely fails or hangs in a sandboxed env — that's fine; we're just confirming the script runs and reaches step1.

(Use `Ctrl+C` to interrupt if it hangs on `npm install`.)

- [ ] **Step 3: Commit**

```bash
git add scripts/install-wizard.js
git commit -m "feat(npm): add install-wizard.js with zh/en i18n and 2-step flow"
```

---

### Task 11: `skills/modelgo-shared/SKILL.md`

**Files:**
- Create: `~/code/modelgo/modelgo-cli/skills/modelgo-shared/SKILL.md`

- [ ] **Step 1: Write the skill file**

```markdown
---
name: modelgo-shared
version: 0.1.0
description: "modelgo-cli setup and troubleshooting. Use when the user first runs modelgo-cli, sees permission/scope errors, needs to update, or asks about installation. Triggers: 安装 modelgo, modelgo install, modelgo upgrade, modelgo update, modelgo permission, modelgo error, modelgo-cli setup, install modelgo-cli."
metadata:
  requires:
    bins: ["modelgo-cli"]
  cliHelp: "modelgo-cli --help"
---

# modelgo-cli setup and troubleshooting

This skill is the shared helper for using `modelgo-cli`. Other `modelgo-*` skills assume the basics here.

## Install or upgrade

Run the official install command. It is idempotent — running it again upgrades to the latest version.

```bash
npx @modelgo/cli@latest install
```

Verify install:

```bash
modelgo-cli --version
```

## v0 framework status

modelgo-cli is currently in v0 framework stage. The only business command available is:

- `modelgo-cli hello [--name NAME]` — print a greeting (demo command, used by the `modelgo-hello` skill)

Business commands (auth, API key management, usage queries, etc.) are not implemented yet. If the user asks for a feature that isn't in `modelgo-cli --help`, tell them it's not available yet and suggest filing an issue at https://github.com/modelgo/modelgo-cli/issues.

## Troubleshooting

- **Command not found after install** — open a new terminal so the updated `PATH` from the npm global bin takes effect. Or run `npm bin -g` to find the install location and add it to `PATH`.
- **Network error during install** — set `https_proxy` if behind a firewall, or use a corporate npm mirror: `npm install -g @modelgo/cli --registry=https://your-mirror/`.
- **`[SECURITY] Checksum mismatch`** — the downloaded binary did not match the expected SHA-256. Do not run it. Report the issue.
```

- [ ] **Step 2: Commit**

```bash
git add skills/modelgo-shared/SKILL.md
git commit -m "feat(skills): add modelgo-shared meta skill for setup and troubleshooting"
```

---

### Task 12: `skills/modelgo-hello/SKILL.md`

**Files:**
- Create: `~/code/modelgo/modelgo-cli/skills/modelgo-hello/SKILL.md`

- [ ] **Step 1: Write the skill file**

```markdown
---
name: modelgo-hello
version: 0.1.0
description: "modelgo-cli demo greeting. Use when the user wants to say hello via modelgo-cli, test the modelgo-cli installation, or verify that modelgo-cli skills are wired up. Triggers: 打招呼, 用 modelgo 打招呼, modelgo hello, hello world, 测试 modelgo, test modelgo-cli."
metadata:
  requires:
    bins: ["modelgo-cli"]
  cliHelp: "modelgo-cli hello --help"
---

# modelgo-cli hello (demo)

This skill exists to verify the full chain: AI agent → skill discovery → `modelgo-cli` binary → output back to the AI.

## When to use

When the user says anything like "have modelgo-cli say hello", "用 modelgo-cli 跟我打个招呼", "test the modelgo CLI", or "verify modelgo skills are wired up".

## How to use

Call `modelgo-cli hello`. The `--name` flag is optional and defaults to `world`.

```bash
modelgo-cli hello              # → "Hello, world!"
modelgo-cli hello --name 渭哲   # → "Hello, 渭哲!"
```

If the user gave their name in the conversation, pass it as `--name <name>`.

After running the command, report the greeting back to the user verbatim — this confirms the end-to-end pipeline works.
```

- [ ] **Step 2: Commit**

```bash
git add skills/modelgo-hello/SKILL.md
git commit -m "feat(skills): add modelgo-hello demo skill"
```

---

### Task 13: `scripts/lint-skills.mjs` + lint run

**Files:**
- Create: `~/code/modelgo/modelgo-cli/scripts/lint-skills.mjs`

- [ ] **Step 1: Add `yaml` dependency**

```bash
cd ~/code/modelgo/modelgo-cli
npm install --save-dev yaml
```

(Engineer note: this is a dev dependency only — used by lint script in CI, not shipped to users.)

Expected: `package.json` now has a `devDependencies` section with `"yaml": "^2.x.x"`. Verify with `cat package.json | grep -A 3 devDependencies`.

- [ ] **Step 2: Write the lint script**

`scripts/lint-skills.mjs`:

```js
#!/usr/bin/env node
// Validate SKILL.md frontmatter for every skill in ./skills/.
// Run: npm run lint:skills

import fs from "node:fs";
import path from "node:path";
import { parse as parseYaml } from "yaml";

const REQUIRED = ["name", "description", "version"];
const SKILLS_DIR = path.join(process.cwd(), "skills");

if (!fs.existsSync(SKILLS_DIR)) {
  console.error(`skills/ directory not found at ${SKILLS_DIR}`);
  process.exit(1);
}

let errors = 0;

function lintSkill(dirName) {
  const skillDir = path.join(SKILLS_DIR, dirName);
  if (!fs.statSync(skillDir).isDirectory()) return;

  const mdPath = path.join(skillDir, "SKILL.md");
  if (!fs.existsSync(mdPath)) {
    console.error(`[${dirName}] missing SKILL.md`);
    errors++;
    return;
  }

  const content = fs.readFileSync(mdPath, "utf8");
  const match = content.match(/^---\n([\s\S]+?)\n---/);
  if (!match) {
    console.error(`[${dirName}] missing YAML frontmatter`);
    errors++;
    return;
  }

  let fm;
  try {
    fm = parseYaml(match[1]);
  } catch (e) {
    console.error(`[${dirName}] invalid YAML: ${e.message}`);
    errors++;
    return;
  }

  for (const key of REQUIRED) {
    if (!fm[key]) {
      console.error(`[${dirName}] missing required field: ${key}`);
      errors++;
    }
  }

  if (fm.name && fm.name !== dirName) {
    console.error(`[${dirName}] frontmatter.name "${fm.name}" does not match directory name`);
    errors++;
  }

  if (fm.description && fm.description.includes("\n")) {
    console.error(`[${dirName}] description must be a single line`);
    errors++;
  }

  if (errors === 0) {
    console.log(`[${dirName}] OK`);
  }
}

for (const entry of fs.readdirSync(SKILLS_DIR)) {
  lintSkill(entry);
}

process.exit(errors > 0 ? 1 : 0);
```

- [ ] **Step 3: Run the lint**

```bash
npm run lint:skills
```

Expected output:
```
[modelgo-hello] OK
[modelgo-shared] OK
```

(Order may vary.)

- [ ] **Step 4: Commit**

```bash
git add scripts/lint-skills.mjs package.json package-lock.json
git commit -m "chore(skills): add lint-skills.mjs to validate SKILL.md frontmatter"
```

---

### Task 14: `.goreleaser.yaml` + local snapshot verification

**Files:**
- Create: `~/code/modelgo/modelgo-cli/.goreleaser.yaml`

- [ ] **Step 1: Write the GoReleaser config**

```yaml
version: 2

project_name: modelgo-cli

before:
  hooks:
    - go mod tidy

builds:
  - id: modelgo-cli
    main: ./cmd/modelgo-cli
    binary: modelgo-cli
    ldflags:
      - -s -w -X github.com/modelgo/modelgo-cli/internal/version.Version=v{{.Version}}
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux, windows]
    goarch: [amd64, arm64]

archives:
  - id: modelgo-cli
    name_template: 'modelgo-cli-{{.Version}}-{{.Os}}-{{.Arch}}'
    format_overrides:
      - goos: windows
        format: zip
    files:
      - LICENSE
      - README.md

checksum:
  name_template: checksums.txt
  algorithm: sha256

release:
  github:
    owner: modelgo
    name: modelgo-cli
  draft: false
  prerelease: auto

changelog:
  use: github
  sort: asc
```

- [ ] **Step 2: Install GoReleaser locally if not present**

```bash
which goreleaser || brew install goreleaser/tap/goreleaser
```

(macOS users have brew; on Linux follow https://goreleaser.com/install/.)

- [ ] **Step 3: Run a snapshot build to verify config**

```bash
cd ~/code/modelgo/modelgo-cli
goreleaser release --snapshot --clean --skip=publish
```

Expected: ~6 archives appear in `dist/`:
```
modelgo-cli-<snapshot>-darwin-amd64.tar.gz
modelgo-cli-<snapshot>-darwin-arm64.tar.gz
modelgo-cli-<snapshot>-linux-amd64.tar.gz
modelgo-cli-<snapshot>-linux-arm64.tar.gz
modelgo-cli-<snapshot>-windows-amd64.zip
modelgo-cli-<snapshot>-windows-arm64.zip
checksums.txt
```

- [ ] **Step 4: Verify checksums.txt format**

```bash
cat dist/checksums.txt | head -3
```

Expected: lines like `<64 hex chars>  modelgo-cli-<ver>-<os>-<arch>.<ext>` — two spaces between hash and filename. This is the format `install.js:getExpectedChecksum` expects.

- [ ] **Step 5: Extract one archive and confirm binary works**

```bash
cd dist
tar -xzf modelgo-cli-*-darwin-arm64.tar.gz -C /tmp 2>/dev/null || \
  tar -xzf modelgo-cli-*-linux-amd64.tar.gz -C /tmp
/tmp/modelgo-cli --version
```

Expected: version string with the snapshot tag.

- [ ] **Step 6: Cleanup and commit**

```bash
cd ~/code/modelgo/modelgo-cli
rm -rf dist /tmp/modelgo-cli
git add .goreleaser.yaml
git commit -m "chore(release): add .goreleaser.yaml with 6-platform matrix"
```

---

### Task 15: `.github/workflows/ci.yml`

**Files:**
- Create: `~/code/modelgo/modelgo-cli/.github/workflows/ci.yml`

- [ ] **Step 1: Write the CI workflow**

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  go:
    name: Go tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go vet ./...
      - run: go test -race ./...

  npm:
    name: npm wrapper tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
      - run: npm install --ignore-scripts
      - run: npm test
      - run: npm run lint:skills
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "chore(ci): add PR workflow running go test, npm test, skills lint"
```

---

### Task 16: `.github/workflows/release.yml`

**Files:**
- Create: `~/code/modelgo/modelgo-cli/.github/workflows/release.yml`

- [ ] **Step 1: Write the release workflow**

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  release:
    name: GoReleaser + npm publish
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          registry-url: "https://registry.npmjs.org"

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Sync checksums.txt back to main
        run: |
          cp dist/checksums.txt checksums.txt
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          if ! git diff --quiet checksums.txt; then
            git add checksums.txt
            git commit -m "chore: update checksums.txt for ${GITHUB_REF_NAME}"
            git push origin HEAD:main
          fi

      - name: Sync package.json version to tag
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          npm version "$VERSION" --no-git-tag-version --allow-same-version

      - name: Publish to npm
        run: npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "chore(release): add tag-triggered workflow for GoReleaser + npm publish"
```

---

### Task 17: README + manual smoke test checklist

**Files:**
- Modify: `~/code/modelgo/modelgo-cli/README.md`
- Create: `~/code/modelgo/modelgo-cli/docs/RELEASING.md`

- [ ] **Step 1: Replace `README.md` with the user-facing version**

```markdown
# modelgo-cli

The official CLI for modelgo. Pairs with AI agent skills (Claude Code, Codex, Gemini CLI, etc.) so AI agents can operate modelgo on your behalf.

> **v0 framework stage.** Business APIs are not wired up yet; the only command is `hello` (a demo to verify the install pipeline).

## Install

```bash
npx @modelgo/cli@latest install
```

This runs an interactive wizard that:

1. Installs `@modelgo/cli` globally via npm (which downloads the Go binary from GitHub Releases).
2. Distributes `modelgo-*` skills to every AI agent installed on your machine (Claude Code, Codex, Gemini CLI, Cursor, and 50+ others — via the `skills` ecosystem).

After install, restart your AI agent (open a new chat / session) and try:

> "Have modelgo-cli say hello to me."

Your AI should find the `modelgo-hello` skill and run `modelgo-cli hello`.

## Direct commands

```bash
modelgo-cli --version
modelgo-cli hello [--name NAME]
modelgo-cli --help
```

## Upgrade

Re-run the installer; it detects an out-of-date install and upgrades in place:

```bash
npx @modelgo/cli@latest install
```

## License

MIT — see [LICENSE](./LICENSE).
```

- [ ] **Step 2: Write `docs/RELEASING.md` with the smoke-test checklist**

```markdown
# Releasing modelgo-cli

## Cutting a release

1. Make sure `main` is green and ahead of the last tag.
2. Tag: `git tag v0.1.0 && git push origin v0.1.0`
3. The `Release` workflow runs:
   - GoReleaser builds 6 platform archives + `checksums.txt`, creates a GitHub Release.
   - `checksums.txt` is committed back to `main` so the next `npm publish` ships with it.
   - `package.json` version is synced to the tag.
   - `npm publish --access public` pushes `@modelgo/cli@<version>` to npmjs.org.

## Manual smoke test (required after every release)

In a clean environment (e.g. fresh Docker container or a machine without modelgo-cli installed):

```bash
docker run --rm -it node:20 bash
# inside the container:
npx @modelgo/cli@latest install --lang=en
which modelgo-cli && modelgo-cli --version       # expect the just-released version
modelgo-cli hello --name smoketest               # expect "Hello, smoketest!"
```

On a developer machine that has Claude Code installed:

```bash
ls ~/.claude/skills/modelgo-shared/SKILL.md      # expect file to exist
ls ~/.claude/skills/modelgo-hello/SKILL.md       # expect file to exist
```

Open a new Claude Code session and ask: "Have modelgo-cli say hello to me." Expect the agent to call `modelgo-cli hello` and report the greeting.

If any step fails, file an issue and consider yanking the release (`npm unpublish @modelgo/cli@<version>` within 72 hours).
```

- [ ] **Step 3: Commit**

```bash
git add README.md docs/RELEASING.md
git commit -m "docs: add user-facing README and release smoke test checklist"
```

---

## Self-review

**Spec coverage check** — every section of `docs/superpowers/specs/2026-05-25-modelgo-cli-design.md` maps to at least one task:

| Spec section | Implementing task(s) |
|---|---|
| §3 命名与许可 | 1, 2, 6 |
| §4 仓库布局 | 1–16 |
| §5.1 Go 二进制 | 3, 4, 5 |
| §5.2 npm wrapper | 6, 7, 8, 9, 10 |
| §5.3 Skills | 11, 12 |
| §5.4 Release 流水线 | 14, 16 |
| §6.1 安装流 | 9 (postinstall guard) + 10 (wizard) |
| §6.2 运行流 | 12 (skill triggers AI) + 5 (binary executes) |
| §7.1 二进制下载错误处理 | 9 (download chain, host allowlist, checksum) |
| §7.2 向导步骤错误处理 | 10 (per-step failure messages) |
| §7.3 非交互 | 10 (`isTTY` branch) |
| §7.4 运行时错误 | 7 (missing-binary error in run.js) |
| §7.5 安全边界 | 8/9 (`execFileSync`, `URL()`, checksums) |
| §8.1 Go 单测 | 3, 4 + 15 (CI runs `go test`) |
| §8.2 npm wrapper 单测 | 8 |
| §8.3 Skills lint | 13 + 15 (CI runs `lint:skills`) |
| §8.4 Release smoke test | 17 (`docs/RELEASING.md`) |

No gaps detected.

**Placeholder scan** — searched for "TBD", "TODO", "fill in", "similar to" — none present. All code blocks are complete.

**Type consistency** — `Greet(name string) string` defined in Task 4 (`internal/hello`), called in Task 5 (`runHello`). `resolveMirrorUrls`, `verifyChecksum`, etc. defined in Task 8, all exported names match their test imports. `archiveName`, `VERSION`, `binDir`, `dest` exported from `install.js` are not consumed by `install-wizard.js` in v0 — they're exported for future tasks; this is fine but worth noting (not a bug).

**Frequent commits** — every task ends in exactly one `git commit`.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-25-modelgo-cli-v0-framework.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration
2. **Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
