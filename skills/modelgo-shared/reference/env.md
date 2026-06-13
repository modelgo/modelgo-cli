# `modelgo env` reference

> Auto-generated from the modelgo CLI `--help` output by `scripts/generate-reference.mjs`.
> Do not edit by hand. Regenerate: `npm run generate:reference` (or `make skills`).

Index: [index.md](index.md)

Run the same command with `--help` in your terminal for identical output.

```text
$ modelgo env --help

modelgo env — manage modelgo environments

USAGE:
    modelgo env <command> [flags]

COMMANDS:
    list                          List built-in and custom envs
    current                       Print the active env name
    use <name>                    Switch the active env
    add <name> --base-url URL     Register or override an env URL
    remove <name>                 Remove a custom env or override

FLAGS:
    --config PATH                 Config file path (default ~/.modelgo/config.json)
    --json                        (list only) Emit JSON output
```
