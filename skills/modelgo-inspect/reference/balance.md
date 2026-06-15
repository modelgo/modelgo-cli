# `modelgo balance` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo balance --help

modelgo balance — view tenant balance and transactions

USAGE:
    modelgo balance                    View current balance overview
    modelgo balance transactions       List billing transactions
    modelgo balance grant              View registration grant status

FLAGS:
    --json              Write structured JSON output
    --type TYPE         (transactions) Filter by type: reserve/settle/release/recharge/adjust/write_off/grant
    --limit N           (transactions) Number of results, max 200 (default 20)
    --cursor CURSOR     (transactions) Keyset cursor: pass the next_cursor from a prior page
    --config PATH       Config file path (default ~/.modelgo/config.json)
    --store PATH        Credential store path (default ~/.modelgo/auth.json)
```
