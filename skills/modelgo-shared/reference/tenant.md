# `modelgo tenant` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo tenant --help

modelgo tenant — manage the active tenant per env

USAGE:
    modelgo tenant <command> [flags]

COMMANDS:
    list                 List logged-in tenants for the env (active marked with *)
    use <slug|id>        Switch the active tenant (use '-' to switch back)

FLAGS:
    --env NAME           Operate on a specific env (default: active env from config)
    --config PATH        Config file path (default ~/.modelgo/config.json)
    --store PATH         Credential store path (default ~/.modelgo/auth.json)
    --remote             (list only) Also fetch all account tenants from the server
```
