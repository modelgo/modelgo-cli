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
