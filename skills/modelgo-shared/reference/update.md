# `modelgo update` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo update --help

modelgo update — update the modelgo CLI to the latest version

USAGE:
    modelgo update [flags]

Detects how modelgo was installed:
  - npm (global):  runs npm install -g @model-go/cli@<latest>, verifies
                   the new binary (rolls back on failure), then re-syncs skills.
  - manual binary: prints the GitHub release download + installer command.

FLAGS:
    --check    Only check for a newer version; do not install
    --force    Reinstall even if already up to date (also re-syncs skills)
    --json     Write structured JSON output (for agents and scripts)
    --help     Show this help
```
