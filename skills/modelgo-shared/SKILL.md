---
name: modelgo-shared
version: 0.1.0
description: "modelgo-cli setup and troubleshooting. Use when the user first runs modelgo-cli, sees permission/scope errors, needs to update, or asks about installation. Triggers: 安装 modelgo, modelgo install, modelgo upgrade, modelgo update, modelgo permission, modelgo error, modelgo-cli setup, install modelgo-cli."
metadata:
  requires:
    bins: ["modelgo-cli"]
  cliHelp: "modelgo-cli --help"
---

# modelgo-cli setup and troubleshooting

This skill is the shared helper for using `modelgo-cli`. Other `modelgo-*` skills assume the basics here.

## Install or upgrade

Run the official install command. It is idempotent — running it again upgrades to the latest version.

```bash
npx @modelgo/cli@latest install
```

Verify install:

```bash
modelgo-cli --version
```

## v0 framework status

modelgo-cli is currently in v0 framework stage. The only business command available is:

- `modelgo-cli hello [--name NAME]` — print a greeting (demo command, used by the `modelgo-hello` skill)

Business commands (auth, API key management, usage queries, etc.) are not implemented yet. If the user asks for a feature that isn't in `modelgo-cli --help`, tell them it's not available yet and suggest filing an issue at https://github.com/modelgo/modelgo-cli/issues.

## Troubleshooting

- **Command not found after install** — open a new terminal so the updated `PATH` from the npm global bin takes effect. Or run `npm bin -g` to find the install location and add it to `PATH`.
- **Network error during install** — set `https_proxy` if behind a firewall, or use a corporate npm mirror: `npm install -g @modelgo/cli --registry=https://your-mirror/`.
- **`[SECURITY] Checksum mismatch`** — the downloaded binary did not match the expected SHA-256. Do not run it. Report the issue.
