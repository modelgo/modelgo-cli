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
