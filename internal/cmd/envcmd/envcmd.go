// Package envcmd implements `modelgo env` subcommands: list, current, use,
// add, remove. The package operates purely on a config file path passed via
// --config; it does not read environment variables.
package envcmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
)

// splitFlagsAndPositionals partitions args into a flag-shaped list (passed
// to fs.Parse) and a list of positional arguments. Without this, Go's
// stdlib flag.Parse would stop at the first non-flag token, preventing
// commands like `env use intl --config /path`.
//
// Bool flags are auto-detected from fs so each subcommand doesn't need to
// keep a separate list in sync.
//
// Limitation: when a string flag's value happens to start with '-' (e.g.
// `--base-url --foo`), the helper still hands the next token off as the
// value. The stdlib flag package then accepts it. Callers relying on
// strict flag-value validation should validate the parsed string after
// fs.Parse returns.
func splitFlagsAndPositionals(args []string, fs *flag.FlagSet) (positional, flagArgs []string) {
	boolFlags := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			boolFlags[f.Name] = true
		}
	})

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			// Everything after -- is positional.
			positional = append(positional, args[i+1:]...)
			return positional, flagArgs
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flagArgs = append(flagArgs, a)
			// Determine flag name (handles both --name and --name=value).
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				// --name=value: value is inline, no lookahead needed.
				continue
			}
			if boolFlags[name] {
				// Bool flag: no value to consume.
				continue
			}
			// Non-bool flag: consume the next token as its value.
			if i+1 < len(args) {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	return positional, flagArgs
}

// Run dispatches an `env` subcommand. args is the list of arguments AFTER
// `modelgo env`. Returns process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "list":
		return runList(args[1:], stdout, stderr)
	case "current":
		return runCurrent(args[1:], stdout, stderr)
	case "use":
		return runUse(args[1:], stdout, stderr)
	case "add":
		return runAdd(args[1:], stdout, stderr)
	case "remove":
		return runRemove(args[1:], stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown env command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path (default ~/.modelgo/config.json)")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	positional, flagArgs := splitFlagsAndPositionals(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "env list: unexpected argument %q\n", positional[0])
		return 2
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env list: %v\n", err)
		return 1
	}
	entries := env.List(cfg)
	if *jsonOut {
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			out = append(out, map[string]any{
				"name":       e.Name,
				"url":        e.URL,
				"built_in":   e.BuiltIn,
				"overridden": e.Overridden,
				"active":     e.Active,
			})
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(out)
		return 0
	}
	for _, e := range entries {
		marker := "  "
		if e.Active {
			marker = "* "
		}
		tags := []string{}
		if e.BuiltIn {
			tags = append(tags, "built-in")
		} else {
			tags = append(tags, "custom")
		}
		if e.Overridden {
			tags = append(tags, "overridden")
		}
		fmt.Fprintf(stdout, "%s%-8s %s  (%s)\n", marker, e.Name, e.URL, strings.Join(tags, ", "))
	}
	return 0
}

func runCurrent(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env current", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	positional, flagArgs := splitFlagsAndPositionals(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "env current: unexpected argument %q\n", positional[0])
		return 2
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env current: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, env.ActiveEnv("", cfg))
	return 0
}

func runUse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env use", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	positional, flagArgs := splitFlagsAndPositionals(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "env use: expected exactly one env name")
		return 2
	}
	name := positional[0]

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env use: %v\n", err)
		return 1
	}
	if _, err := env.Resolve(name, cfg); err != nil {
		fmt.Fprintf(stderr, "env use: %v\n", err)
		return 1
	}
	cfg.CurrentEnv = name
	if err := saveConfig(*configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "env use: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Switched to env %q\n", name)
	return 0
}

func runAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	baseURL := fs.String("base-url", "", "API base URL for this env (required)")
	positional, flagArgs := splitFlagsAndPositionals(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "env add: expected exactly one env name")
		return 2
	}
	name := positional[0]
	if *baseURL == "" {
		fmt.Fprintln(stderr, "env add: --base-url is required")
		return 2
	}
	parsed, err := url.Parse(*baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		fmt.Fprintf(stderr, "env add: invalid --base-url: %q\n", *baseURL)
		return 1
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		fmt.Fprintf(stderr, "env add: --base-url scheme must be http or https, got %q\n", parsed.Scheme)
		return 1
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env add: %v\n", err)
		return 1
	}
	if cfg.Envs == nil {
		cfg.Envs = map[string]config.EnvEntry{}
	}
	cfg.Envs[name] = config.EnvEntry{BaseURL: strings.TrimRight(*baseURL, "/")}
	if err := saveConfig(*configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "env add: %v\n", err)
		return 1
	}
	if env.IsBuiltIn(name) {
		fmt.Fprintf(stdout, "Overrode built-in env %q with %s\n", name, parsed.String())
	} else {
		fmt.Fprintf(stdout, "Added env %q → %s\n", name, parsed.String())
	}
	return 0
}

func runRemove(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env remove", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	positional, flagArgs := splitFlagsAndPositionals(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "env remove: expected exactly one env name")
		return 2
	}
	name := positional[0]

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env remove: %v\n", err)
		return 1
	}
	if cfg.CurrentEnv == name {
		fmt.Fprintf(stderr, "env remove: cannot remove the active environment %q. Run 'modelgo env use <other>' first.\n", name)
		return 1
	}
	if _, ok := cfg.Envs[name]; !ok {
		if env.IsBuiltIn(name) {
			fmt.Fprintf(stdout, "Env %q has no override to remove\n", name)
			return 0
		}
		fmt.Fprintf(stderr, "env remove: unknown env %q\n", name)
		return 1
	}
	delete(cfg.Envs, name)
	if err := saveConfig(*configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "env remove: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Removed env %q\n", name)
	return 0
}

func loadConfig(path string) (config.Config, error) {
	if path == "" {
		path = config.DefaultPath()
	}
	return config.Load(path)
}

func saveConfig(path string, cfg config.Config) error {
	if path == "" {
		path = config.DefaultPath()
	}
	return config.Save(path, cfg)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo env — manage modelgo environments

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
    --json                        (list only) Emit JSON output`)
}
