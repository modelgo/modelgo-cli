#!/usr/bin/env node
// Sync the `version:` frontmatter of every skills/*/SKILL.md from the single
// source of truth: the `version` field in package.json (the published npm
// version, which is also the git release tag and what `npx skills add` ships).
//
// Run: npm run sync:skills   (also part of `npm run sync:skill-assets`)
// Enforced by: npm run lint:skills (fails the build on drift).
//
// Mirrors bailian-cli's tools/sync-skill-metadata.ts, adapted for modelgo's
// multi-skill layout and its top-level (un-nested) `version:` field.

import fs from "node:fs";
import path from "node:path";

const ROOT = path.resolve(import.meta.dirname, "..");
const PKG_PATH = path.join(ROOT, "package.json");
const SKILLS_DIR = path.join(ROOT, "skills");

// Matches a top-level `version:` line inside the YAML frontmatter, with or
// without surrounding quotes, e.g. `version: 0.1.0` or `version: "0.1.0"`.
// Anchored to column 0 (no leading whitespace) so an indented, nested
// version-named sub-key (e.g. under `metadata:`) can never match.
const VERSION_LINE_RE = /^(version:[ \t]*)["']?[^"'\r\n]*["']?([ \t]*)$/m;
const VERSION_LINE_GLOBAL_RE = /^version:[ \t]*["']?[^"'\r\n]*["']?[ \t]*$/gm;

const { version } = JSON.parse(fs.readFileSync(PKG_PATH, "utf8"));
if (!version) {
  console.error("package.json has no version field");
  process.exit(1);
}

if (!fs.existsSync(SKILLS_DIR)) {
  console.error(`skills/ directory not found at ${SKILLS_DIR}`);
  process.exit(1);
}

let changed = 0;
let failed = 0;

for (const entry of fs.readdirSync(SKILLS_DIR)) {
  const skillDir = path.join(SKILLS_DIR, entry);
  if (!fs.statSync(skillDir).isDirectory()) continue;

  const mdPath = path.join(skillDir, "SKILL.md");
  if (!fs.existsSync(mdPath)) continue;

  const body = fs.readFileSync(mdPath, "utf8");
  const lines = body.split(/\r?\n/);
  if (lines[0] !== "---") {
    console.error(`[${entry}] SKILL.md must start with --- YAML frontmatter`);
    failed++;
    continue;
  }
  const closeIdx = lines.findIndex((line, i) => i > 0 && line === "---");
  if (closeIdx === -1) {
    console.error(`[${entry}] SKILL.md missing closing --- frontmatter delimiter`);
    failed++;
    continue;
  }

  const frontmatter = lines.slice(0, closeIdx + 1).join("\n");
  const rest = lines.slice(closeIdx + 1).join("\n");

  const topLevelMatches = frontmatter.match(VERSION_LINE_GLOBAL_RE) || [];
  if (topLevelMatches.length !== 1) {
    console.error(
      `[${entry}] expected exactly one top-level \`version:\` line in frontmatter, found ${topLevelMatches.length}`,
    );
    failed++;
    continue;
  }

  const updatedFrontmatter = frontmatter.replace(
    VERSION_LINE_RE,
    (_m, g1, g2) => `${g1}${version}${g2}`,
  );
  const newBody = updatedFrontmatter + "\n" + rest;

  if (newBody !== body) {
    fs.writeFileSync(mdPath, newBody, "utf8");
    console.log(`[${entry}] version → ${version}`);
    changed++;
  } else {
    console.log(`[${entry}] already ${version}`);
  }
}

if (failed > 0) process.exit(1);
if (changed === 0) console.log(`All skills already at ${version}`);
