# `modelgo auth` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo auth --help

modelgo auth — authentication commands

USAGE:
    modelgo auth <command> [flags]

COMMANDS:
    login    Log in with device authorization
    status   Show local auth status
    logout   Clear local auth credentials

FLAGS:
    --env NAME       Operate on a specific env (default: active env from config)
    --tenant SLUG    (logout) Clear a single tenant instead of the whole env
    --config PATH    Config file path (default ~/.modelgo/config.json)
    --store PATH     Credential store path (default ~/.modelgo/auth.json)
    --all            (logout) Clear all envs
```

```text
$ modelgo auth login --help

modelgo auth login — device authorization login

USAGE:
    modelgo auth login [flags]

DEFAULT FLOW:
    1. Request a device_code and verification_url
    2. Print the URL for the user to open in their browser
    3. Block and poll until authorization completes

AGENT FLOW:
    For non-streaming agent harnesses, use:
      modelgo auth login --no-wait --json
    Return verification_url to the user exactly as printed, end the turn, then
    after the user confirms approval run:
      modelgo auth login --device-code <DEVICE_CODE>

FLAGS:
    --env NAME       Env to log into (default: active env from config)
    --scope SCOPE    Space- or comma-separated scopes to request
    --config PATH    Config file path (default ~/.modelgo/config.json)
    --store PATH     Credential store path (default ~/.modelgo/auth.json)
    --no-wait        Print device authorization URL and return immediately
    --device-code    Poll an existing device code
    --json           Write structured JSON output (NDJSON in blocking mode)
```
