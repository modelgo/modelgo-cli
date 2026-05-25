# modelgo-cli

The official CLI for modelgo. Pairs with AI agent skills (Claude Code, Codex, Gemini CLI, etc.) so AI agents can operate modelgo on your behalf.

> **v0 framework stage.** Business APIs are not wired up yet; the only command is `hello` (a demo to verify the install pipeline).

## Install

```bash
npx @modelgo/cli@latest install
```

This runs an interactive wizard that:

1. Installs `@modelgo/cli` globally via npm (which downloads the Go binary from GitHub Releases).
2. Distributes `modelgo-*` skills to every AI agent installed on your machine (Claude Code, Codex, Gemini CLI, Cursor, and 50+ others — via the `skills` ecosystem).

After install, restart your AI agent (open a new chat / session) and try:

> "Have modelgo-cli say hello to me."

Your AI should find the `modelgo-hello` skill and run `modelgo-cli hello`.

## Direct commands

```bash
modelgo-cli --version
modelgo-cli hello [--name NAME]
modelgo-cli --help
```

## Upgrade

Re-run the installer; it detects an out-of-date install and upgrades in place:

```bash
npx @modelgo/cli@latest install
```

## License

MIT — see [LICENSE](./LICENSE).
