// Command modelgo is the modelgo CLI entrypoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	cliauth "github.com/modelgo/modelgo-cli/internal/auth"
	"github.com/modelgo/modelgo-cli/internal/cmd/envcmd"
	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
	"github.com/modelgo/modelgo-cli/internal/hello"
	"github.com/modelgo/modelgo-cli/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Fprintln(stdout, version.Version)
	case "--help", "-h":
		printUsage(stdout)
	case "hello":
		return runHello(args[1:], stdout, stderr)
	case "auth":
		return runAuth(args[1:], stdout, stderr)
	case "env":
		return envcmd.Run(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
	return 0
}

func runHello(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hello", flag.ExitOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "world", "name to greet")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fmt.Fprintln(stdout, hello.Greet(*name))
	return 0
}

func runAuth(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printAuthUsage(stderr)
		return 2
	}
	switch args[0] {
	case "login":
		return runAuthLogin(args[1:], stdout, stderr)
	case "status":
		return runAuthStatus(args[1:], stdout, stderr)
	case "logout":
		return runAuthLogout(args[1:], stdout, stderr)
	case "--help", "-h":
		printAuthUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown auth command: %s\n\n", args[0])
		printAuthUsage(stderr)
		return 2
	}
}

// resolveEnvAndURL loads the config file at configPath, picks the active env
// (respecting an explicit --env flag), and resolves it to a base URL.
func resolveEnvAndURL(envFlag, configPath string, stderr io.Writer) (envName, baseURL string, ok bool) {
	if configPath == "" {
		configPath = config.DefaultPath()
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return "", "", false
	}
	envName = env.ActiveEnv(envFlag, cfg)
	url, err := env.Resolve(envName, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return "", "", false
	}
	return envName, url, true
}

func runAuthLogin(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to log into (default: active env from config)")
	configPath := fs.String("config", "", "config file path (default ~/.modelgo/config.json)")
	store := fs.String("store", "", "credential store path (default ~/.modelgo/auth.json)")
	scope := fs.String("scope", "", "space- or comma-separated scopes to request")
	noWait := fs.Bool("no-wait", false, "print device authorization URL and return immediately")
	deviceCode := fs.String("device-code", "", "poll an existing device code")
	jsonOut := fs.Bool("json", false, "write structured JSON output (NDJSON: device-code object, then authenticated object)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	envName, baseURL, ok := resolveEnvAndURL(*envFlag, *configPath, stderr)
	if !ok {
		return 1
	}

	ctx := context.Background()
	loginOpts := cliauth.Options{
		Env:        envName,
		BaseURL:    baseURL,
		Scope:      *scope,
		DeviceCode: *deviceCode,
		StorePath:  *store,
	}
	if *deviceCode == "" {
		loginOpts.NoWait = true
	}
	result, err := cliauth.Login(ctx, loginOpts)
	if err != nil {
		fmt.Fprintf(stderr, "auth login failed: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(loginJSON(result, *noWait))
		if *noWait || *deviceCode != "" || result.Authenticated {
			return 0
		}
	} else if !result.Authenticated {
		fmt.Fprintf(stdout, "Open this URL to authorize modelgo:\n%s\n\nUser code: %s\n", result.VerificationURL, result.UserCode)
		if *noWait {
			fmt.Fprintf(stdout, "\nAfter approving, run:\nmodelgo auth login --device-code %s\n", result.DeviceCode)
		}
	}
	if *noWait || result.Authenticated {
		if result.Authenticated && !*jsonOut {
			fmt.Fprintf(stdout, "Logged in as %s (env %s)\n", result.AccountID, result.Env)
		}
		return 0
	}

	if *deviceCode == "" {
		fmt.Fprintln(stderr, "Waiting for authorization...")
		result, err = cliauth.Login(ctx, cliauth.Options{
			Env:        envName,
			BaseURL:    baseURL,
			DeviceCode: result.DeviceCode,
			StorePath:  *store,
		})
		if err != nil {
			fmt.Fprintf(stderr, "auth login failed: %v\n", err)
			return 1
		}
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(loginJSON(result, false))
	} else {
		fmt.Fprintf(stdout, "\nLogged in as %s (env %s)\n", result.AccountID, result.Env)
	}
	return 0
}

func loginJSON(result *cliauth.LoginResult, noWait bool) map[string]any {
	if result.Authenticated {
		return map[string]any{
			"authenticated": true,
			"env":           result.Env,
			"account_id":    result.AccountID,
			"tenant_id":     result.TenantID,
			"expires_at":    result.ExpiresAt.Format(time.RFC3339Nano),
		}
	}
	out := map[string]any{
		"env":              result.Env,
		"verification_url": result.VerificationURL,
		"device_code":      result.DeviceCode,
		"user_code":        result.UserCode,
		"expires_in":       result.ExpiresIn,
		"interval":         result.Interval,
	}
	if noWait {
		out["hint"] = fmt.Sprintf("Show verification_url to the user exactly as returned. After approval, run: modelgo auth login --device-code %s", result.DeviceCode)
	}
	return out
}

func runAuthStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to check (default: active env from config)")
	configPath := fs.String("config", "", "config file path")
	store := fs.String("store", "", "credential store path")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	envName := *envFlag
	if envName == "" {
		// First-run users have no config file and config.Load returns
		// (empty, nil) — so this only surfaces real parse errors.
		cfg, err := config.Load(configPathOrDefault(*configPath))
		if err != nil {
			fmt.Fprintf(stderr, "auth status: load config: %v\n", err)
			return 1
		}
		envName = env.ActiveEnv("", cfg)
	}

	ok, cred, err := cliauth.Status(envName, *store)
	if err != nil {
		fmt.Fprintf(stderr, "auth status failed: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if !ok {
			_ = enc.Encode(map[string]any{"logged_in": false, "env": envName})
		} else {
			_ = enc.Encode(map[string]any{
				"logged_in":  true,
				"env":        envName,
				"account_id": cred.AccountID,
				"tenant_id":  cred.TenantID,
				"expires_at": cred.ExpiresAt.Format(time.RFC3339Nano),
			})
		}
	} else if ok {
		tenant := cred.TenantSlug
		if tenant == "" {
			tenant = cred.TenantID
		}
		fmt.Fprintf(stdout, "Logged in as %s (env %s, tenant %s)\n", cred.AccountID, envName, tenant)
	} else {
		fmt.Fprintf(stdout, "Not logged in (env %s)\n", envName)
	}
	if !ok {
		return 1
	}
	return 0
}

func runAuthLogout(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to log out of (default: active env from config)")
	tenantFlag := fs.String("tenant", "", "log out of a single tenant (slug or id) instead of the whole env")
	configPath := fs.String("config", "", "config file path")
	store := fs.String("store", "", "credential store path")
	all := fs.Bool("all", false, "log out of all envs")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *all {
		if err := cliauth.LogoutAll(*store); err != nil {
			fmt.Fprintf(stderr, "auth logout failed: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Logged out of all envs")
		return 0
	}

	envName := *envFlag
	if envName == "" {
		cfg, err := config.Load(configPathOrDefault(*configPath))
		if err != nil {
			fmt.Fprintf(stderr, "auth logout: load config: %v\n", err)
			return 1
		}
		envName = env.ActiveEnv("", cfg)
	}

	tenantID := ""
	if *tenantFlag != "" {
		id, err := cliauth.ResolveTenantID(envName, *tenantFlag, *store)
		if err != nil {
			fmt.Fprintf(stderr, "auth logout: %v\n", err)
			return 1
		}
		tenantID = id
	}
	if err := cliauth.Logout(envName, tenantID, *store); err != nil {
		fmt.Fprintf(stderr, "auth logout failed: %v\n", err)
		return 1
	}
	if tenantID != "" {
		fmt.Fprintf(stdout, "Logged out of tenant %s (env %s)\n", tenantID, envName)
	} else {
		fmt.Fprintf(stdout, "Logged out of env %s\n", envName)
	}
	return 0
}

func configPathOrDefault(p string) string {
	if p == "" {
		return config.DefaultPath()
	}
	return p
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo — the official modelgo CLI

USAGE:
    modelgo <command> [flags]

COMMANDS:
    auth login            Log in with device authorization
    auth status           Show local auth status
    auth logout           Clear local auth credentials
    env list              List built-in and custom envs
    env current           Print the active env
    env use <name>        Switch the active env
    env add <name>        Register or override an env URL
    env remove <name>     Remove a custom env or override
    hello [--name NAME]   Print a greeting
    --version, -v         Print the version
    --help, -h            Show this help`)
}

func printAuthUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo auth — authentication commands

USAGE:
    modelgo auth <command> [flags]

COMMANDS:
    login    Log in with device authorization
    status   Show local auth status
    logout   Clear local auth credentials

FLAGS:
    --env NAME       Operate on a specific env (default: active env from config)
    --tenant SLUG    (logout) Clear a single tenant instead of the whole env
    --config PATH    Config file path (default ~/.modelgo/config.json)
    --store PATH     Credential store path (default ~/.modelgo/auth.json)
    --all            (logout) Clear all envs`)
}
