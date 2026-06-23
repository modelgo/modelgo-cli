# `modelgo CLI overview` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo --help

modelgo — the official modelgo CLI

USAGE:
    modelgo <command> [flags]

COMMANDS:
    auth login            Log in with device authorization
    auth status           Show local auth status
    auth logout           Clear local auth credentials
    env list              List built-in and custom envs
    env current           Print the active env
    env use <name>        Switch the active env
    env add <name>        Register or override an env URL
    env remove <name>     Remove a custom env or override
    tenant list           List logged-in tenants for the active env
    tenant use <slug|id>  Switch the active tenant (use '-' to go back)
    balance               View tenant balance and transactions
    permissions           View account permissions
    logs                  Query call logs and usage statistics
    pay                   Manage x402 pay-per-call payment profile
    chat                  Call a chat model (/v1/chat/completions)
    models                List available models (/v1/models)
    embeddings            Create embeddings (/v1/embeddings)
    call                  Raw passthrough to any /v1/* model endpoint
    key                   Manage the stored model API key (per env)
    update                Update modelgo to the latest version
    --version, -v         Print the version
    --help, -h            Show this help

GLOBAL FLAGS:
    --tenant <slug|id>    Before the subcommand: use a specific logged-in tenant
                          for one call (balance, permissions, logs only)
```
