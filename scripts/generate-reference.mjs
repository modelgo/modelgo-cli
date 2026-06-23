#!/usr/bin/env node
// Generator: scrapes `modelgo <group> --help` from the built binary and writes,
// for each skill that owns those command groups:
//   - skills/<skill>/reference/<group>.md  — the authoritative per-group help
//   - skills/<skill>/reference/index.md     — quick index + global conventions
//
// Committed to git; consumed by the modelgo-* Agent Skills so an agent can read
// the command reference without executing the CLI, always in sync with the
// binary (the help text is the CLI's own).
//
// Run: npm run generate:reference   (also part of `npm run sync:skill-assets`)
// Requires a built binary: set MODELGO_BIN, or run `make build` first
// (the generator falls back to ./bin/modelgo).
//
// Mirrors bailian-cli's tools/generate-reference.ts. modelgo has no machine-
// readable command catalog (commands use hand-written flag sets + usage text),
// so this scrapes the hand-written group `--help` rather than a data structure.

import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const ROOT = path.resolve(import.meta.dirname, "..");
const SKILLS_DIR = path.join(ROOT, "skills");
const BIN = process.env.MODELGO_BIN || path.join(ROOT, "bin", "modelgo");

const BANNER =
  "> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.\n" +
  "> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).";

// Each reference page belongs to exactly one skill (skills are installed
// independently, so each carries the reference for the commands it documents).
// `help` is the list of `--help` invocations whose output is concatenated.
const PAGES = [
  {
    skill: "modelgo-shared",
    group: "overview",
    title: "modelgo CLI overview",
    help: [[]],
  },
  {
    skill: "modelgo-shared",
    group: "auth",
    title: "modelgo auth",
    help: [["auth"], ["auth", "login"]],
  },
  { skill: "modelgo-shared", group: "update", title: "modelgo update", help: [["update"]] },
  { skill: "modelgo-shared", group: "env", title: "modelgo env", help: [["env"]] },
  { skill: "modelgo-shared", group: "tenant", title: "modelgo tenant", help: [["tenant"]] },
  { skill: "modelgo-inspect", group: "balance", title: "modelgo balance", help: [["balance"]] },
  {
    skill: "modelgo-inspect",
    group: "permissions",
    title: "modelgo permissions",
    help: [["permissions"]],
  },
  { skill: "modelgo-inspect", group: "logs", title: "modelgo logs", help: [["logs"]] },
  {
    // `pay` uses Go's per-subcommand flag package, so the group `--help` lists
    // only subcommand names — each subcommand's flags live in its own `--help`
    // (printed to stderr with exit 2; help() captures that). Scrape them all.
    skill: "modelgo-x402",
    group: "pay",
    title: "modelgo pay",
    help: [["pay"], ["pay", "methods"], ["pay", "set"], ["pay", "status"], ["pay", "header"], ["pay", "request"]],
  },
  { skill: "modelgo-call", group: "chat", title: "modelgo chat", help: [["chat"]] },
  { skill: "modelgo-call", group: "models", title: "modelgo models", help: [["models"]] },
  { skill: "modelgo-call", group: "embeddings", title: "modelgo embeddings", help: [["embeddings"]] },
  { skill: "modelgo-call", group: "call", title: "modelgo call", help: [["call"]] },
  {
    // `key` uses Go's per-subcommand flag package; the group `--help` lists only
    // subcommand names, so scrape each subcommand for its own flags.
    skill: "modelgo-call",
    group: "key",
    title: "modelgo key",
    help: [["key"], ["key", "set"], ["key", "show"], ["key", "remove"]],
  },
];

function ensureBinary() {
  if (!fs.existsSync(BIN)) {
    console.error(
      `modelgo binary not found at ${BIN}.\n` +
        `Build it first (\`make build\`) or set MODELGO_BIN to its path.`,
    );
    process.exit(1);
  }
}

// Run `modelgo <args> --help` and return stdout (help text), falling back to
// stderr. --help exits 0 for groups, but execFileSync throws on non-zero, so we
// capture both streams regardless of exit code.
function help(args) {
  try {
    const out = execFileSync(BIN, [...args, "--help"], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
    return out.trimEnd();
  } catch (e) {
    const text = (e.stdout || "") + (e.stderr || "");
    return text.trimEnd();
  }
}

function buildPage(page) {
  const blocks = page.help.map((args) => {
    const cmd = ["modelgo", ...args].join(" ");
    return ["```text", `$ ${cmd} --help`, "", help(args), "```"].join("\n");
  });
  return [
    `# \`${page.title}\` reference`,
    "",
    BANNER,
    "",
    "Index: [index.md](index.md)",
    "",
    "Run the same command with `--help` in your terminal for identical output.",
    "",
    blocks.join("\n\n"),
    "",
  ].join("\n");
}

function buildIndex(skill, pages) {
  const lines = [
    `# ${skill} command reference`,
    "",
    BANNER,
    "",
    "Per-group details are in the sibling `<group>.md` files in this directory.",
    "",
    "## Pages",
    "",
    "| Group | Reference |",
    "| --- | --- |",
  ];
  for (const p of pages) {
    lines.push(`| \`${p.group}\` | [${p.group}.md](${p.group}.md) |`);
  }
  lines.push(
    "",
    "## Global conventions",
    "",
    "- `--json` — structured JSON on **stdout** (success only). Errors always go to **stderr** as plain text.",
    "- `--env <name>` — accepted only by `auth`, `tenant`, and `pay request`. `balance`/`permissions`/`logs` always use the active env (switch with `modelgo env use <name>`); passing `--env` to them is a usage error (exit 2).",
    "- `--tenant <slug|id>` — global flag (before the subcommand) selecting which logged-in tenant authenticates the call, for `balance`/`permissions`/`logs`. Unknown tenant → exit 1; on any other command → usage error (exit 2). `modelgo tenant use <slug|id>` changes the default tenant.",
    "- `--config PATH` / `--store PATH` — override the config (`~/.modelgo/config.json`) and credential store (`~/.modelgo/auth.json`).",
    "",
    "## Exit codes",
    "",
    "| Code | Meaning |",
    "| --- | --- |",
    "| 0 | Success |",
    "| 1 | Runtime error (auth/permission/network/API/CLI) — see stderr message |",
    "| 2 | Usage error (bad flag, unknown subcommand, missing argument) |",
    "",
  );
  return lines.join("\n");
}

function writeReference() {
  ensureBinary();

  const bySkill = new Map();
  for (const page of PAGES) {
    if (!bySkill.has(page.skill)) bySkill.set(page.skill, []);
    bySkill.get(page.skill).push(page);
  }

  let total = 0;
  for (const [skill, pages] of bySkill) {
    const refDir = path.join(SKILLS_DIR, skill, "reference");
    if (!fs.existsSync(path.join(SKILLS_DIR, skill))) {
      console.error(`skill directory not found: ${skill}`);
      process.exit(1);
    }
    fs.mkdirSync(refDir, { recursive: true });

    // Remove stale generated files from previous runs.
    for (const name of fs.readdirSync(refDir)) {
      if (name.endsWith(".md")) fs.rmSync(path.join(refDir, name));
    }

    for (const page of pages) {
      fs.writeFileSync(path.join(refDir, `${page.group}.md`), buildPage(page), "utf8");
      total++;
    }
    fs.writeFileSync(path.join(refDir, "index.md"), buildIndex(skill, pages), "utf8");
    console.log(`[${skill}] wrote reference/index.md + ${pages.length} group files`);
  }
  console.log(`Generated ${total} reference pages across ${bySkill.size} skills`);
}

writeReference();
