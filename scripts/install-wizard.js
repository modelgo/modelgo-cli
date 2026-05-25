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
