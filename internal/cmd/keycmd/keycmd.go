// Package keycmd implements `modelgo key`: store, show, and remove the per-env
// model API key (mgk_...) used by the model commands. The key is written to
// ~/.modelgo/config.json (0600), under api_keys keyed by env name.
package keycmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
	"github.com/modelgo/modelgo-cli/internal/modelapi"
)

// Run dispatches a `key` subcommand. args is everything after `modelgo key`.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "set":
		return runSet(args[1:], stdin, stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "remove", "rm", "unset":
		return runRemove(args[1:], stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown key command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runSet(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("key set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to store the key for (default: active env)")
	configPath := fs.String("config", "", "config file path")
	// The KEY is positional and may precede flags (`key set mgk_x --env cn`);
	// hoist flags ahead of it so they parse regardless of order.
	if err := fs.Parse(hoistFlags(args)); err != nil {
		return 2
	}

	var key string
	if rest := fs.Args(); len(rest) > 0 {
		key = strings.TrimSpace(rest[0])
	} else {
		b, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "key set: %v\n", err)
			return 1
		}
		key = strings.TrimSpace(string(b))
	}
	if key == "" {
		fmt.Fprintln(stderr, "key set: no key provided (pass it as an argument or via stdin)")
		return 2
	}

	path := configPathOrDefault(*configPath)
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "key set: load config: %v\n", err)
		return 1
	}
	envName := env.ActiveEnv(*envFlag, cfg)
	if cfg.APIKeys == nil {
		cfg.APIKeys = map[string]string{}
	}
	cfg.APIKeys[envName] = key
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintf(stderr, "key set: save config: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Stored API key for env %s (%s)\n", envName, maskKey(key))
	return 0
}

func runShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("key show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to show the key for (default: active env)")
	configPath := fs.String("config", "", "config file path")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(configPathOrDefault(*configPath))
	if err != nil {
		fmt.Fprintf(stderr, "key show: load config: %v\n", err)
		return 1
	}
	envName := env.ActiveEnv(*envFlag, cfg)
	key := strings.TrimSpace(cfg.APIKeys[envName])
	source := "stored"
	// Reflect the effective resolution so users understand which key would be used.
	if resolved, rerr := modelapi.ResolveAPIKey("", envName, cfg); rerr == nil && resolved != key {
		key = resolved
		source = "env:" + modelapi.EnvAPIKey
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{
			"env":    envName,
			"set":    key != "",
			"source": source,
			"masked": maskKey(key),
		})
		if key == "" {
			return 1
		}
		return 0
	}

	if key == "" {
		fmt.Fprintf(stdout, "No API key for env %s. Run `modelgo key set` or set %s.\n", envName, modelapi.EnvAPIKey)
		return 1
	}
	fmt.Fprintf(stdout, "env %s: %s (%s)\n", envName, maskKey(key), source)
	return 0
}

func runRemove(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("key remove", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to remove the key for (default: active env)")
	configPath := fs.String("config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	path := configPathOrDefault(*configPath)
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "key remove: load config: %v\n", err)
		return 1
	}
	envName := env.ActiveEnv(*envFlag, cfg)
	if _, ok := cfg.APIKeys[envName]; !ok {
		fmt.Fprintf(stdout, "No stored API key for env %s.\n", envName)
		return 0
	}
	delete(cfg.APIKeys, envName)
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintf(stderr, "key remove: save config: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Removed API key for env %s\n", envName)
	return 0
}

// keyBoolFlags are the `key` flags that take no value.
var keyBoolFlags = map[string]bool{"--json": true, "-json": true, "--help": true, "-h": true}

// hoistFlags reorders args so flags (and their values) precede positionals,
// letting the standard flag parser see every flag even when the positional KEY
// comes first.
func hoistFlags(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		if strings.Contains(a, "=") || keyBoolFlags[a] {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}

// maskKey shows the key prefix and last 4 chars, hiding the secret middle.
func maskKey(key string) string {
	if key == "" {
		return "(none)"
	}
	if len(key) <= 12 {
		return "****"
	}
	return key[:8] + "…" + key[len(key)-4:]
}

func configPathOrDefault(p string) string {
	if p == "" {
		return config.DefaultPath()
	}
	return p
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo key — manage the stored model API key (per env)

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
    Get an API key from the ModelGo console; the CLI does not mint keys.`)
}
