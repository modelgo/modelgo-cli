# `modelgo key` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo key --help

modelgo key — manage the stored model API key (per env)

USAGE:
    modelgo key set [KEY]      Store a key for the active env (reads stdin if omitted)
    modelgo key show           Show the masked key that would be used
    modelgo key remove         Delete the stored key for the active env

FLAGS:
    --env NAME       Operate on a specific env (default: active env)
    --config PATH    Config file path (default ~/.modelgo/config.json)
    --json           (show) Write structured JSON output

NOTES:
    Resolution precedence for model commands: --api-key > MODELGO_API_KEY > stored key.
    Get an API key from the ModelGo console; the CLI does not mint keys.
```

```text
$ modelgo key set --help

Usage of key set:
  -config string
    	config file path
  -env string
    	env to store the key for (default: active env)
```

```text
$ modelgo key show --help

Usage of key show:
  -config string
    	config file path
  -env string
    	env to show the key for (default: active env)
  -json
    	write structured JSON output
```

```text
$ modelgo key remove --help

Usage of key remove:
  -config string
    	config file path
  -env string
    	env to remove the key for (default: active env)
```
