# `modelgo chat` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo chat --help

modelgo chat — call a chat model (OpenAI-compatible /v1/chat/completions)

USAGE:
    modelgo chat --model MODEL [flags] [PROMPT...]
    echo "PROMPT" | modelgo chat --model MODEL

FLAGS:
    --model MODEL       Model id (required)
    --system TEXT       System prompt
    --stream            Stream the response token-by-token
    --image SRC         Image URL or local file for vision models (repeatable)
    --max-tokens N      Max output tokens
    --temperature T     Sampling temperature (0–2)
    --api-key KEY       Model API key (else MODELGO_API_KEY or stored key)
    --env NAME          Env to call (default: active env)
    --config PATH       Config file path
    --json              Print the raw JSON response (NDJSON frames when --stream)

EXAMPLES:
    modelgo chat --model gpt-4o "Write a haiku about Go"
    modelgo chat --model gpt-4o --stream "Explain channels"
    modelgo chat --model gpt-4o --image photo.png "What is in this image?"
    cat prompt.txt | modelgo chat --model claude-opus-4-8
```
