// Command modelgo-cli is the modelgo CLI entrypoint.
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

func runAuthLogin(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "", "modelgo-permissions base URL")
	store := fs.String("store", "", "credential store path")
	scope := fs.String("scope", "", "space- or comma-separated scopes to request")
	noWait := fs.Bool("no-wait", false, "print device authorization URL and return immediately")
	deviceCode := fs.String("device-code", "", "poll an existing device code")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()
	loginOpts := cliauth.Options{
		BaseURL:    *baseURL,
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
		fmt.Fprintf(stdout, "Open this URL to authorize modelgo-cli:\n%s\n\nUser code: %s\n", result.VerificationURL, result.UserCode)
		if *noWait {
			fmt.Fprintf(stdout, "\nAfter approving, run:\nmodelgo-cli auth login --device-code %s\n", result.DeviceCode)
		}
	}
	if *noWait || result.Authenticated {
		if result.Authenticated && !*jsonOut {
			fmt.Fprintf(stdout, "Logged in as %s\n", result.AccountID)
		}
		return 0
	}

	if *deviceCode == "" {
		fmt.Fprintln(stderr, "Waiting for authorization...")
		result, err = cliauth.Login(ctx, cliauth.Options{
			BaseURL:    *baseURL,
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
		fmt.Fprintf(stdout, "\nLogged in as %s\n", result.AccountID)
	}
	return 0
}

func loginJSON(result *cliauth.LoginResult, noWait bool) map[string]any {
	if result.Authenticated {
		return map[string]any{
			"authenticated": true,
			"account_id":    result.AccountID,
			"tenant_id":     result.TenantID,
			"expires_at":    result.ExpiresAt.Format(time.RFC3339Nano),
		}
	}
	out := map[string]any{
		"verification_url": result.VerificationURL,
		"device_code":      result.DeviceCode,
		"user_code":        result.UserCode,
		"expires_in":       result.ExpiresIn,
		"interval":         result.Interval,
	}
	if noWait {
		out["hint"] = fmt.Sprintf("Show verification_url to the user exactly as returned. After approval, run: modelgo-cli auth login --device-code %s", result.DeviceCode)
	}
	return out
}

func runAuthStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	store := fs.String("store", "", "credential store path")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ok, cred, err := cliauth.Status(*store)
	if err != nil {
		fmt.Fprintf(stderr, "auth status failed: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if !ok {
			_ = enc.Encode(map[string]any{"logged_in": false})
		} else {
			_ = enc.Encode(map[string]any{"logged_in": true, "account_id": cred.AccountID, "tenant_id": cred.TenantID, "expires_at": cred.ExpiresAt.Format(time.RFC3339Nano)})
		}
	} else if ok {
		fmt.Fprintf(stdout, "Logged in as %s\n", cred.AccountID)
	} else {
		fmt.Fprintln(stdout, "Not logged in")
	}
	if !ok {
		return 1
	}
	return 0
}

func runAuthLogout(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	store := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := cliauth.Logout(*store); err != nil {
		fmt.Fprintf(stderr, "auth logout failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "Logged out")
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo-cli — the official modelgo CLI

USAGE:
    modelgo-cli <command> [flags]

COMMANDS:
    auth login            Log in with device authorization
    auth status           Show local auth status
    auth logout           Clear local auth credentials
    hello [--name NAME]   Print a greeting
    --version, -v         Print the version
    --help, -h            Show this help`)
}

func printAuthUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo-cli auth — authentication commands

USAGE:
    modelgo-cli auth <command> [flags]

COMMANDS:
    login                 Log in with device authorization
    status                Show local auth status
    logout                Clear local auth credentials`)
}
