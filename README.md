# modelgo-cli

The official CLI for modelgo. Pairs with AI agent skills (Claude Code, Codex, Gemini CLI, etc.) so AI agents can operate modelgo on your behalf.

> **v0 framework stage.** Device-flow `auth` commands are available. API key, usage, and model gateway commands are not wired up yet.

## Install

```bash
npx @model-go/cli@latest install
```

This runs an interactive wizard that:

1. Installs `@model-go/cli` globally via npm (which downloads the Go binary from GitHub Releases as `modelgo`).
2. Distributes `modelgo-*` skills to every AI agent installed on your machine (Claude Code, Trae, Trae CN, and other agents in the [skills](https://www.npmjs.com/package/skills) ecosystem).

After install, restart your AI agent (open a new chat / session) and try:

> "Check my modelgo login status."

Your AI should find the `modelgo-shared` skill and run `modelgo auth status`.

## Direct commands

```bash
modelgo --version
modelgo auth login
modelgo auth status
modelgo auth logout
modelgo --help
```

`modelgo auth login` without `--no-wait` is a blocking command: it prints the
authorization URL, then keeps polling until the browser approval completes or
the device code expires.

### Environments

The CLI talks to a named environment. Two are built in:

| Env    | Base URL                     |
| ------ | ---------------------------- |
| `cn`   | `https://api.modelgo.com`    |
| `intl` | `https://api.modelgo.global` |

The active environment defaults to `cn`. Switch it, or register your own:

```bash
modelgo env list                 # show built-in + custom envs (active marked with *)
modelgo env current              # print the active env
modelgo env use intl             # switch the active env
modelgo env add <name> --base-url https://...   # register or override an env URL
modelgo env remove <name>        # remove a custom env or override
```

Environment definitions and the active selection live in `~/.modelgo/config.json`.
Credentials are stored per-env in `~/.modelgo/auth.json`, so switching environments
preserves each login. There are no environment variables to set.

Auth commands operate on the active env; pass `--env <name>` to target a different one:

```bash
modelgo auth login --env intl
modelgo auth status --env intl
modelgo auth logout --all          # clear every env
```

### Non-streaming agent flows

For AI-agent harnesses that cannot stream intermediate output to the user, use:

```bash
modelgo auth login --no-wait --json
modelgo auth login --device-code <DEVICE_CODE>
```

Do not show the URL and then immediately block on `--device-code` in the same
turn. In final-message-only harnesses, that pattern prevents the user from ever
seeing the URL before the agent starts waiting.

> **`--json` emits newline-delimited JSON (NDJSON).** In the default waiting
> flow (`modelgo auth login --json` without `--no-wait`), stdout receives two
> JSON objects: first the device-code object (with `verification_url`), then
> the authenticated object once approved. Parse line by line — don't
> `JSON.parse` the whole stream. With `--no-wait`, only the device-code object
> is printed.

## Upgrade

Re-run the installer; it detects an out-of-date install and upgrades in place:

```bash
npx @model-go/cli@latest install
```

## License

MIT — see [LICENSE](./LICENSE).
