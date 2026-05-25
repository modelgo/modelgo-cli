// Copyright (c) 2026 modelgo
// SPDX-License-Identifier: MIT

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");
const os = require("os");
const crypto = require("crypto");

const VERSION = require("../package.json").version;
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
  const split = (v) => {
    const idx = v.indexOf("-");
    return idx === -1 ? [v, ""] : [v.slice(0, idx), v.slice(idx + 1)];
  };
  const [coreA, preA] = split(a);
  const [coreB, preB] = split(b);
  const pa = coreA.split(".").map(Number);
  const pb = coreB.split(".").map(Number);
  for (let i = 0; i < 3; i++) {
    if ((pa[i] || 0) < (pb[i] || 0)) return true;
    if ((pa[i] || 0) > (pb[i] || 0)) return false;
  }
  if (preA === preB) return false;
  if (preA === "") return false;
  if (preB === "") return true;
  const idsA = preA.split(".");
  const idsB = preB.split(".");
  const n = Math.max(idsA.length, idsB.length);
  for (let i = 0; i < n; i++) {
    const x = idsA[i];
    const y = idsB[i];
    if (x === undefined) return true;
    if (y === undefined) return false;
    const numX = /^\d+$/.test(x) ? parseInt(x, 10) : null;
    const numY = /^\d+$/.test(y) ? parseInt(y, 10) : null;
    if (numX !== null && numY !== null) {
      if (numX < numY) return true;
      if (numX > numY) return false;
    } else if (numX !== null) {
      return true;
    } else if (numY !== null) {
      return false;
    } else {
      if (x < y) return true;
      if (x > y) return false;
    }
  }
  return false;
}

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
      `  npm install -g @model-go/cli\n\n` +
      `  # 2. Point to a corporate npm mirror that proxies /-/binary/${NAME}/...:\n` +
      `  npm install -g @model-go/cli --registry=https://your-corp-mirror/`
    );
    process.exit(1);
  }
}

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
