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
      `Please reinstall: npm install -g @model-go/cli`
    );
    process.exit(1);
  }

  try {
    execFileSync(binPath, args, { stdio: "inherit" });
  } catch (e) {
    process.exit(typeof e.status === "number" ? e.status : 1);
  }
}
