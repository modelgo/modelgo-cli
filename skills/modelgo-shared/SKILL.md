---
name: modelgo-shared
version: 0.1.0
description: "modelgo-cli setup and troubleshooting. Use when the user first runs modelgo, sees permission/scope errors, needs to update, or asks about installation. Triggers: 安装 modelgo, modelgo install, modelgo upgrade, modelgo update, modelgo permission, modelgo error, modelgo setup, install modelgo-cli."
metadata:
  requires:
    bins: ["modelgo"]
  cliHelp: "modelgo --help"
---

# modelgo-cli setup and troubleshooting

This skill is the shared helper for using the `modelgo` CLI. Other `modelgo-*` skills assume the basics here.

## Install or upgrade

Run the official install command. It is idempotent — running it again upgrades to the latest version.

```bash
npx @model-go/cli@latest install
```

Verify install:

```bash
modelgo --version
```

## Commands

Currently available commands:

- `modelgo hello [--name NAME]` — print a greeting (demo command, used by the `modelgo-hello` skill)
- `modelgo auth login [--env NAME] [--scope SCOPE]` — log in with ModelGo device authorization
- `modelgo auth login --no-wait --json` — get a verification URL and device code without blocking
- `modelgo auth login --device-code CODE` — resume polling after the user approves the URL from a prior `--no-wait` run
- `modelgo auth status` — show local login status
- `modelgo auth logout` — clear local credentials (`--all` clears every env)
- `modelgo env list` / `modelgo env use <name>` / `modelgo env add <name> --base-url URL` — manage environments (`cn`, `intl`, or custom)

API key management, usage queries, and model gateway commands are not implemented yet. If the user asks for a feature that isn't in `modelgo --help`, tell them it's not available yet and suggest filing an issue at https://github.com/modelgo/modelgo-cli/issues.

For non-streaming agent harnesses, use `modelgo auth login --no-wait --json`, return the `verification_url` to the user exactly as printed, then after the user confirms approval run `modelgo auth login --device-code <device_code>`.

## Troubleshooting

- **Command not found after install** — open a new terminal so the updated `PATH` from the npm global bin takes effect. Or run `npm bin -g` to find the install location and add it to `PATH`.
- **Network error during install** — set `https_proxy` if behind a firewall, or use a corporate npm mirror: `npm install -g @model-go/cli --registry=https://your-mirror/`.
- **`[SECURITY] Checksum mismatch`** — the downloaded binary did not match the expected SHA-256. Do not run it. Report the issue.
