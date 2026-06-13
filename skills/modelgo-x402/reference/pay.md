# `modelgo pay` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo pay --help

Usage: modelgo pay <subcommand>

Manage the x402 (pay-per-call) payment profile used to satisfy a gateway 402.

Subcommands:
  methods            List the payment channels the gateway can advertise.
  set                Store a payment profile (network/scheme/credential).
                     Credential via --token or the MODELGO_PAYMENT_TOKEN env var.
  status             Show the stored payment profile (credential redacted).
  header             Print the X-Payment-Protocol + X-PAYMENT headers to attach
                     to a request (for an agent / manual retry).
  request            Call a model API with x402 enabled; on domestic 402,
                     prepare an Alipay skill handoff.

Examples:
  modelgo pay set --method alipay --network alipay:cnpc --token <agent_token>
  modelgo pay header --json
  modelgo pay request --path /v1/chat/completions --method POST --data '{"model":"gpt-4o","messages":[]}'
```

```text
$ modelgo pay methods --help

Usage of pay methods:
  -json
    	output JSON
```

```text
$ modelgo pay set --help

Usage of pay set:
  -method string
    	payment method: alipay | blockchain (default "alipay")
  -network string
    	CAIP-2 network (default "alipay:cnpc")
  -payer string
    	optional payer reference
  -scheme string
    	x402 scheme (default "upto")
  -token string
    	agent payment credential token (or set MODELGO_PAYMENT_TOKEN)
```

```text
$ modelgo pay status --help

Usage of pay status:
  -json
    	output JSON
```

```text
$ modelgo pay header --help

Usage of pay header:
  -json
    	output JSON
```

```text
$ modelgo pay request --help

Usage of pay request:
  -config string
    	config file path (default ~/.modelgo/config.json)
  -data string
    	request body
  -data-file string
    	file containing request body
  -env string
    	env to use (default: active env from config)
  -header value
    	additional request header KEY:VALUE; repeatable
  -intent string
    	original user request summary for payment handoff
  -json
    	output JSON for agents
  -method string
    	HTTP method (default "GET")
  -network string
    	preferred x402 network
  -path string
    	model API path relative to the env base URL, e.g. /v1/chat/completions
  -payment-dir string
    	directory for the generated PAYMENT-REQUIRED file (default ".")
  -url string
    	absolute model API URL
```
