---
name: modelgo-hello
version: 0.1.0
description: "modelgo-cli demo greeting. Use when the user wants to say hello via modelgo-cli, test the modelgo-cli installation, or verify that modelgo-cli skills are wired up. Triggers: 打招呼, 用 modelgo 打招呼, modelgo hello, hello world, 测试 modelgo, test modelgo-cli."
metadata:
  requires:
    bins: ["modelgo-cli"]
  cliHelp: "modelgo-cli hello --help"
---

# modelgo-cli hello (demo)

This skill exists to verify the full chain: AI agent → skill discovery → `modelgo-cli` binary → output back to the AI.

## When to use

When the user says anything like "have modelgo-cli say hello", "用 modelgo-cli 跟我打个招呼", "test the modelgo CLI", or "verify modelgo skills are wired up".

## How to use

Call `modelgo-cli hello`. The `--name` flag is optional and defaults to `world`.

```bash
modelgo-cli hello              # → "Hello, world!"
modelgo-cli hello --name 渭哲   # → "Hello, 渭哲!"
```

If the user gave their name in the conversation, pass it as `--name <name>`.

After running the command, report the greeting back to the user verbatim — this confirms the end-to-end pipeline works.
