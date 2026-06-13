// Package permissionscmd implements `modelgo permissions` — view the current
// account's granted permissions and accessible menus.
package permissionscmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/modelgo/modelgo-cli/internal/apiclient"
)

// Run dispatches the `permissions` command. args is everything after `modelgo
// permissions`. Returns the process exit code.
func Run(args []string, tenant string, stdout, stderr io.Writer) int {
	// Handle --help before flag parsing so we get a custom usage message.
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			printUsage(stdout)
			return 0
		}
	}

	fs := flag.NewFlagSet("permissions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	configPath := fs.String("config", "", "config file path (default ~/.modelgo/config.json)")
	storePath := fs.String("store", "", "credential store path (default ~/.modelgo/auth.json)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var opts []apiclient.Option
	if *configPath != "" {
		opts = append(opts, apiclient.WithConfigPath(*configPath))
	}
	if *storePath != "" {
		opts = append(opts, apiclient.WithStorePath(*storePath))
	}

	client, err := apiclient.NewFromConfig(tenant, opts...)
	if err != nil {
		fmt.Fprintf(stderr, "permissions: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var resp permissionsResponse
	if err := client.Get(ctx, "account/permissions", &resp); err != nil {
		fmt.Fprintf(stderr, "permissions: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(resp)
		return 0
	}

	displayTenant := client.TenantSlug
	if displayTenant == "" {
		displayTenant = client.TenantID
	}
	fmt.Fprintf(stdout, "Permissions (tenant: %s)\n", displayTenant)

	// Granted
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Granted:")
	if len(resp.Granted) == 0 {
		fmt.Fprintln(stdout, "  (none)")
	} else {
		fmt.Fprintf(stdout, "  %s\n", strings.Join(resp.Granted, "    "))
	}

	// Menus
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Menus:")
	if len(resp.Menus) == 0 {
		fmt.Fprintln(stdout, "  (none)")
	} else {
		for _, m := range resp.Menus {
			fmt.Fprintf(stdout, "  %s\n", m.Label)
			printMenuChildren(stdout, m.Children, "    ")
		}
	}

	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo permissions — view account permissions

USAGE:
    modelgo permissions

FLAGS:
    --json              Write structured JSON output
    --config PATH       Config file path (default ~/.modelgo/config.json)
    --store PATH        Credential store path (default ~/.modelgo/auth.json)`)
}

func printMenuChildren(w io.Writer, children []menuItem, prefix string) {
	for i, child := range children {
		connector := "├─"
		if i == len(children)-1 {
			connector = "└─"
		}
		fmt.Fprintf(w, "%s%s %s\n", prefix, connector, child.Label)
		if len(child.Children) > 0 {
			childPrefix := prefix
			if i < len(children)-1 {
				childPrefix += "│  "
			} else {
				childPrefix += "   "
			}
			printMenuChildren(w, child.Children, childPrefix)
		}
	}
}

// ── API types ───────────────────────────────────────────────────────────────

type permissionsResponse struct {
	Granted []string   `json:"granted"`
	Menus   []menuItem `json:"menus"`
}

type menuItem struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Children []menuItem `json:"children"`
}
