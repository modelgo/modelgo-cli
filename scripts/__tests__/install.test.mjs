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

test("semverLessThan: core X.Y.Z numeric comparison", () => {
  assert.equal(semverLessThan("1.0.0", "1.0.1"), true);
  assert.equal(semverLessThan("1.0.1", "1.0.0"), false);
  assert.equal(semverLessThan("1.0.0", "1.0.0"), false);
  assert.equal(semverLessThan("1.10.0", "1.9.0"), false);
});

test("semverLessThan: pre-release ranks below release at same X.Y.Z", () => {
  assert.equal(semverLessThan("2.0.0-beta", "2.0.0"), true);
  assert.equal(semverLessThan("2.0.0", "2.0.0-beta"), false);
  assert.equal(semverLessThan("0.1.0-rc.3", "0.1.0"), true);
  assert.equal(semverLessThan("0.1.0", "0.1.0-rc.3"), false);
});

test("semverLessThan: pre-release identifiers compared correctly", () => {
  // Numeric identifiers compared numerically (rc.10 > rc.9, not lex)
  assert.equal(semverLessThan("0.1.0-rc.9", "0.1.0-rc.10"), true);
  assert.equal(semverLessThan("0.1.0-rc.10", "0.1.0-rc.9"), false);
  assert.equal(semverLessThan("0.1.0-rc.3", "0.1.0-rc.4"), true);
  // Identical pre-release
  assert.equal(semverLessThan("0.1.0-rc.3", "0.1.0-rc.3"), false);
  // Shorter pre-release identifier list ranks below longer (alpha < alpha.1)
  assert.equal(semverLessThan("1.0.0-alpha", "1.0.0-alpha.1"), true);
  // Numeric identifier < alphanumeric (rc.1 < rc.alpha)
  assert.equal(semverLessThan("1.0.0-rc.1", "1.0.0-rc.alpha"), true);
});

test("semverLessThan: core difference dominates pre-release suffix", () => {
  assert.equal(semverLessThan("0.1.0-rc.3", "0.1.1"), true);
  assert.equal(semverLessThan("0.1.1", "0.1.0-rc.3"), false);
});
