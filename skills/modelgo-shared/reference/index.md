# modelgo-shared command reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Per-group details are in the sibling `<group>.md` files in this directory.

## Pages

| Group | Reference |
| --- | --- |
| `overview` | [overview.md](overview.md) |
| `auth` | [auth.md](auth.md) |
| `env` | [env.md](env.md) |
| `tenant` | [tenant.md](tenant.md) |

## Global conventions

- `--json` — structured JSON on **stdout** (success only). Errors always go to **stderr** as plain text.
- `--env <name>` — accepted only by `auth`, `tenant`, and `pay request`. `balance`/`permissions`/`logs` always use the active env (switch with `modelgo env use <name>`); passing `--env` to them is a usage error (exit 2).
- `--tenant <slug|id>` — global flag (before the subcommand) selecting which logged-in tenant authenticates the call, for `balance`/`permissions`/`logs`. Unknown tenant → exit 1; on any other command → usage error (exit 2). `modelgo tenant use <slug|id>` changes the default tenant.
- `--config PATH` / `--store PATH` — override the config (`~/.modelgo/config.json`) and credential store (`~/.modelgo/auth.json`).

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Runtime error (auth/permission/network/API/CLI) — see stderr message |
| 2 | Usage error (bad flag, unknown subcommand, missing argument) |
