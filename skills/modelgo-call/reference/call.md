# `modelgo call` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo call --help

modelgo call — raw passthrough to any /v1/* gateway endpoint

USAGE:
    modelgo call <path> [flags]

Covers every OpenAI-compatible endpoint, including multimodal ones not wrapped
by chat/models/embeddings: images, audio, anthropic messages, rerank, etc.

FLAGS:
    --method METHOD     HTTP method (default POST with a body, GET without)
    --data JSON         Request body as a literal JSON string
    --data-file FILE    Request body from a file ("-" = stdin)
    --header "K: V"     Extra request header (repeatable)
    --stream            Stream the response body verbatim
    --api-key KEY       Model API key (else MODELGO_API_KEY or stored key)
    --env NAME          Env to call (default: active env)
    --config PATH       Config file path

EXAMPLES:
    modelgo call /v1/images/generations --data '{"model":"dall-e-3","prompt":"a cat"}'
    modelgo call /v1/messages --data-file anthropic_req.json
    modelgo call /v1/audio/speech --data '{"model":"tts-1","input":"hi","voice":"alloy"}' > out.mp3
    modelgo call /v1/embeddings/multimodal --data-file req.json
    echo '{"model":"gpt-4o","messages":[...]}' | modelgo call /v1/chat/completions --stream
```
