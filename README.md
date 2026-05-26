# modelgo-cli

The official CLI for modelgo. Pairs with AI agent skills (Claude Code, Codex, Gemini CLI, etc.) so AI agents can operate modelgo on your behalf.

> **v0 framework stage.** The demo `hello` command and device-flow `auth` commands are available. API key, usage, and model gateway commands are not wired up yet.

## Install

```bash
npx @model-go/cli@latest install
```

This runs an interactive wizard that:

1. Installs `@model-go/cli` globally via npm (which downloads the Go binary from GitHub Releases).
2. Distributes `modelgo-*` skills to every AI agent installed on your machine (Claude Code, Trae, Trae CN, and other agents in the [skills](https://www.npmjs.com/package/skills) ecosystem).

After install, restart your AI agent (open a new chat / session) and try:

> "Have modelgo-cli say hello to me."

Your AI should find the `modelgo-hello` skill and run `modelgo-cli hello`.

## Direct commands

```bash
modelgo-cli --version
modelgo-cli auth login --base-url https://permissions.example.com
modelgo-cli auth status
modelgo-cli auth logout
modelgo-cli hello [--name NAME]
modelgo-cli --help
```

For AI-agent flows that cannot stream intermediate output to the user, use:

```bash
modelgo-cli auth login --base-url https://permissions.example.com --no-wait --json
modelgo-cli auth login --base-url https://permissions.example.com --device-code <DEVICE_CODE>
```

## Upgrade

Re-run the installer; it detects an out-of-date install and upgrades in place:

```bash
npx @model-go/cli@latest install
```

## License

MIT — see [LICENSE](./LICENSE).
