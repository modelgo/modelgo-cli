#!/usr/bin/env node
// Copyright (c) 2026 modelgo
// SPDX-License-Identifier: MIT

const { execFileSync, execFile } = require("child_process");
const p = require("@clack/prompts");
const { semverLessThan } = require("./install.js");

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

function getGloballyInstalledVersion() {
  try {
    const out = runSilent("npm", ["list", "-g", PKG], { timeout: 15000 });
    const match = out.toString().match(/@(\d+\.\d+\.\d+[^\s]*)/);
    return match ? match[1] : null;
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

function reportStart(isInteractive, message) {
  if (!isInteractive) {
    console.log(message);
    return null;
  }
  const s = p.spinner();
  s.start(message);
  return s;
}

function reportStop(isInteractive, spinner, message) {
  if (isInteractive && spinner) spinner.stop(message);
  else console.log(message);
}

function reportFail(isInteractive, spinner, message, err) {
  if (isInteractive && spinner) spinner.stop(message);
  else console.error(message);
  const stderr = err && err.stderr ? err.stderr.toString().trim() : "";
  if (stderr) console.error("\n" + stderr);
}

async function stepInstallGlobally(msg, isInteractive) {
  const installedVer = getGloballyInstalledVersion();
  const latestVer = getLatestVersion();
  const needsUpgrade = installedVer && latestVer && semverLessThan(installedVer, latestVer);

  if (installedVer && !needsUpgrade) {
    const skipMsg = fmt(msg.step1Skip, installedVer);
    if (isInteractive) p.log.info(skipMsg);
    else console.log(skipMsg);
    return;
  }

  const startMsg = needsUpgrade
    ? fmt(msg.step1Upgrade, PKG, installedVer, latestVer)
    : fmt(msg.step1, PKG);
  const doneMsg = needsUpgrade ? fmt(msg.step1Upgraded, latestVer) : msg.step1Done;

  const s = reportStart(isInteractive, startMsg);
  try {
    await runSilentAsync("npm", ["install", "-g", PKG], { timeout: 120000 });
    reportStop(isInteractive, s, doneMsg);
  } catch (err) {
    reportFail(isInteractive, s, fmt(msg.step1Fail, PKG), err);
    process.exit(1);
  }
}

async function stepInstallSkills(msg, isInteractive) {
  // Always re-run `skills add` so new skills added in newer modelgo-cli
  // versions actually land on the user's machine. The skills CLI is
  // idempotent for existing skills (re-copies content) and additive for
  // new ones; the slight extra GitHub fetch is worth keeping skill
  // distribution in sync with the latest published bundle.
  const s = reportStart(isInteractive, msg.step2Spinner);
  try {
    await runSilentAsync("npx", ["-y", "skills", "add", SKILLS_REPO, "-y", "-g"], { timeout: 120000 });
    reportStop(isInteractive, s, msg.step2Done);
  } catch (err) {
    reportFail(isInteractive, s, fmt(msg.step2Fail, SKILLS_REPO), err);
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
    await stepInstallGlobally(msg, isInteractive);
    await stepInstallSkills(msg, isInteractive);
    p.outro(msg.done);
  } else {
    console.log(msg.setup);
    await stepInstallGlobally(msg, isInteractive);
    await stepInstallSkills(msg, isInteractive);
    console.log(msg.nonTtyHint);
  }
}

main().catch((err) => {
  p.cancel("Unexpected error: " + (err.message || err));
  process.exit(1);
});
