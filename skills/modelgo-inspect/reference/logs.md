# `modelgo logs` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo logs --help

modelgo logs — query call logs and usage statistics

USAGE:
    modelgo logs                         List recent call logs
    modelgo logs <request-id>            View call detail
    modelgo logs <request-id> payload    View request/response payload
    modelgo logs stats                   Call statistics by group
    modelgo logs usage                   Usage summary

FLAGS:
    --json                 Write structured JSON output
    --limit N              (list) Number of results, max 200 (default 20)
    --preset DURATION      (list) Time preset: 1h, 24h, 7d
    --status STATUS        (list) Filter by status: succeeded/failed (aliases: success, error)
    --model MODEL          (list/stats) Filter by model name
    --workspace ID         (list/stats) Filter by workspace ID
    --api-key ID           (list) Filter by API key ID
    --kind KIND            (payload) Payload kind: request or response (default response)
    --from DATE            (stats/usage) Start date (YYYY-MM-DD)
    --to DATE              (stats/usage) End date (YYYY-MM-DD)
    --group-by DIM         (stats) Group by: none/model/provider/workspace/creator/api_key (default model)
    --granularity G        (stats) Time granularity: hour/day (default day)
    --config PATH          Config file path (default ~/.modelgo/config.json)
    --store PATH           Credential store path (default ~/.modelgo/auth.json)
```
