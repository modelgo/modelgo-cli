---
name: modelgo-hello
version: 0.1.0
description: "modelgo-cli demo greeting. Use when the user wants to say hello via modelgo, test the modelgo installation, or verify that modelgo-cli skills are wired up. Triggers: 打招呼, 用 modelgo 打招呼, modelgo hello, hello world, 测试 modelgo, test modelgo-cli."
metadata:
  requires:
    bins: ["modelgo"]
  cliHelp: "modelgo hello --help"
---

# modelgo hello (demo)

This skill exists to verify the full chain: AI agent → skill discovery → `modelgo` binary → output back to the AI.

## When to use

When the user says anything like "have modelgo say hello", "用 modelgo 跟我打个招呼", "test the modelgo CLI", or "verify modelgo skills are wired up".

## How to use

Call `modelgo hello`. The `--name` flag is optional and defaults to `world`.

```bash
modelgo hello              # → "Hello, world!"
modelgo hello --name 渭哲   # → "Hello, 渭哲!"
```

If the user gave their name in the conversation, pass it as `--name <name>`.

After running the command, report the greeting back to the user verbatim — this confirms the end-to-end pipeline works.
