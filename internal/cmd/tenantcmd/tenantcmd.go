// Package tenantcmd implements `modelgo tenant` subcommands: list, use. The
// CLI stores one credential per (env, tenant) plus an active pointer (see
// package auth); these commands inspect and flip that pointer without
// re-running device login.
package tenantcmd

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/modelgo/modelgo-cli/internal/auth"
	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
)

// splitFlagsAndPositionals mirrors envcmd's helper so `tenant use acme --env cn`
// parses the same way regardless of token order.
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
			positional = append(positional, args[i+1:]...)
			return positional, flagArgs
		}
		// "-" alone is the use-previous sentinel, treat as positional.
		if strings.HasPrefix(a, "-") && a != "-" {
			flagArgs = append(flagArgs, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				continue
			}
			if boolFlags[name] {
				continue
			}
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

// Run dispatches a `tenant` subcommand. args is everything after `modelgo
// tenant`. Returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "list":
		return runListCmd(args[1:], stdout, stderr)
	case "use":
		return runUseCmd(args[1:], stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown tenant command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

// resolveEnv resolves the env name from an explicit --env flag or the active
// env in the config file.
func resolveEnv(envFlag, configPath string) (string, error) {
	if envFlag != "" {
		return envFlag, nil
	}
	if configPath == "" {
		configPath = config.DefaultPath()
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return env.ActiveEnv("", cfg), nil
}

func runListCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tenant list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to list tenants for (default: active env from config)")
	configPath := fs.String("config", "", "config file path")
	store := fs.String("store", "", "credential store path")
	remote := fs.Bool("remote", false, "also fetch all account tenants from the server")
	positional, flagArgs := splitFlagsAndPositionals(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "tenant list: unexpected argument %q\n", positional[0])
		return 2
	}
	envName, err := resolveEnv(*envFlag, *configPath)
	if err != nil {
		fmt.Fprintf(stderr, "tenant list: %v\n", err)
		return 1
	}
	if *remote {
		if err := runListRemote(stdout, stderr, envName, *store); err != nil {
			fmt.Fprintf(stderr, "tenant list --remote: %v\n", err)
			return 1
		}
		return 0
	}
	if err := runList(stdout, envName, *store); err != nil {
		fmt.Fprintf(stderr, "tenant list: %v\n", err)
		return 1
	}
	return 0
}

// runList prints all logged-in tenants for an env, prefixing the active one
// with "* " and the rest with "  ". Columns: tenant_id  slug  name.
func runList(w io.Writer, envName, path string) error {
	creds, active, err := auth.ListTenants(envName, path)
	if err != nil {
		return err
	}
	if len(creds) == 0 {
		fmt.Fprintf(w, "No tenants logged in for env %q. Run `modelgo auth login`.\n", envName)
		return nil
	}
	for _, c := range creds {
		marker := "  "
		if c.TenantID == active {
			marker = "* "
		}
		fmt.Fprintf(w, "%s%-26s %-16s %s\n", marker, c.TenantID, dash(c.TenantSlug), dash(c.TenantName))
	}
	return nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func runUseCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tenant use", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to switch tenant within (default: active env from config)")
	configPath := fs.String("config", "", "config file path")
	store := fs.String("store", "", "credential store path")
	positional, flagArgs := splitFlagsAndPositionals(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "tenant use: expected exactly one tenant (slug, id, or '-')")
		return 2
	}
	target := positional[0]
	envName, err := resolveEnv(*envFlag, *configPath)
	if err != nil {
		fmt.Fprintf(stderr, "tenant use: %v\n", err)
		return 1
	}

	if target == "-" {
		if err := auth.UsePreviousTenant(envName, *store); err != nil {
			fmt.Fprintf(stderr, "tenant use: %v\n", err)
			return 1
		}
	} else {
		tenantID, err := auth.ResolveTenantID(envName, target, *store)
		if err != nil {
			fmt.Fprintf(stderr, "tenant use: %v\n", err)
			return 1
		}
		if err := auth.UseTenant(envName, tenantID, *store); err != nil {
			fmt.Fprintf(stderr, "tenant use: %v\n", err)
			return 1
		}
	}
	active, err := auth.LoadActive(envName, *store)
	if err != nil {
		fmt.Fprintf(stderr, "tenant use: %v\n", err)
		return 1
	}
	label := active.TenantSlug
	if label == "" {
		label = active.TenantID
	}
	fmt.Fprintf(stdout, "Switched to tenant %s (env %s)\n", label, envName)
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo tenant — manage the active tenant per env

USAGE:
    modelgo tenant <command> [flags]

COMMANDS:
    list                 List logged-in tenants for the env (active marked with *)
    use <slug|id>        Switch the active tenant (use '-' to switch back)

FLAGS:
    --env NAME           Operate on a specific env (default: active env from config)
    --config PATH        Config file path (default ~/.modelgo/config.json)
    --store PATH         Credential store path (default ~/.modelgo/auth.json)
    --remote             (list only) Also fetch all account tenants from the server`)
}

// runListRemote is implemented in tenantcmd_remote.go.
