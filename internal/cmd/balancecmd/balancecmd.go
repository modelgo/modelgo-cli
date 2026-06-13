// Package balancecmd implements `modelgo balance` subcommands: default (overview),
// transactions, grant. All requests go through the gateway /open/v1/* proxy.
package balancecmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/modelgo/modelgo-cli/internal/apiclient"
)

// Run dispatches a `balance` subcommand. args is everything after `modelgo
// balance`. Returns the process exit code.
func Run(args []string, tenant string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		// No subcommand: show the balance overview (mirrors `logs` → list).
		// The binary's own usage text advertises bare `balance` as the overview.
		return runOverview(args, tenant, stdout, stderr)
	}
	switch args[0] {
	case "transactions":
		return runTransactions(args[1:], tenant, stdout, stderr)
	case "grant":
		return runGrant(args[1:], tenant, stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		// Unknown token — if it looks like a flag, treat as the default
		// (overview) subcommand with flags. Otherwise report unknown.
		if strings.HasPrefix(args[0], "-") {
			return runOverview(args, tenant, stdout, stderr)
		}
		fmt.Fprintf(stderr, "unknown balance command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

// ── balance (overview) ──────────────────────────────────────────────────────

type balanceResponse struct {
	TenantID            string    `json:"tenant_id"`
	Balance             float64   `json:"balance"`
	FrozenBalance       float64   `json:"frozen_balance"`
	Currency            string    `json:"currency"`
	Status              string    `json:"status"`
	LowBalanceThreshold float64   `json:"low_balance_threshold"`
	AutoTopupEnabled    bool      `json:"auto_topup_enabled"`
	AutoTopupAmount     float64   `json:"auto_topup_amount"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func runOverview(args []string, tenant string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("balance", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	configPath := fs.String("config", "", "config file path (default ~/.modelgo/config.json)")
	storePath := fs.String("store", "", "credential store path (default ~/.modelgo/auth.json)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig(tenant, opts...)
	if err != nil {
		fmt.Fprintf(stderr, "balance: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("tenants/%s/balance", url.PathEscape(client.TenantID))
	var resp balanceResponse
	if err := client.Get(ctx, path, &resp); err != nil {
		fmt.Fprintf(stderr, "balance: %v\n", err)
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
	symbol := currencySymbol(resp.Currency)
	fmt.Fprintf(stdout, "Balance (tenant: %s)\n", displayTenant)
	fmt.Fprintf(stdout, "  Available:    %s %s\n", symbol, formatAmount(resp.Balance, resp.Currency))
	fmt.Fprintf(stdout, "  Frozen:       %s %s\n", symbol, formatAmount(resp.FrozenBalance, resp.Currency))
	fmt.Fprintf(stdout, "  Currency:     %s\n", resp.Currency)
	fmt.Fprintf(stdout, "  Status:       %s\n", resp.Status)
	topup := "disabled"
	if resp.AutoTopupEnabled {
		topup = fmt.Sprintf("enabled (%s %s)", symbol, formatAmount(resp.AutoTopupAmount, resp.Currency))
	}
	fmt.Fprintf(stdout, "  Auto Top-up:  %s\n", topup)
	if !resp.UpdatedAt.IsZero() {
		fmt.Fprintf(stdout, "  Updated:      %s\n", resp.UpdatedAt.Format("2006-01-02 15:04:05"))
	}
	return 0
}

// ── balance transactions ────────────────────────────────────────────────────

type transaction struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func runTransactions(args []string, tenant string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("balance transactions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	typeFilter := fs.String("type", "", "filter by type (consumption/recharge/refund/grant)")
	limit := fs.Int("limit", 20, "number of results (max 100)")
	before := fs.String("before", "", "keyset pagination cursor")
	configPath := fs.String("config", "", "config file path")
	storePath := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig(tenant, opts...)
	if err != nil {
		fmt.Fprintf(stderr, "balance transactions: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := url.Values{}
	if *typeFilter != "" {
		params.Set("type", *typeFilter)
	}
	if *limit > 0 && *limit <= 100 {
		params.Set("limit", fmt.Sprintf("%d", *limit))
	}
	if *before != "" {
		params.Set("before", *before)
	}

	path := fmt.Sprintf("tenants/%s/transactions", url.PathEscape(client.TenantID))
	var txns []transaction
	if err := client.GetWithQuery(ctx, path, params, &txns); err != nil {
		fmt.Fprintf(stderr, "balance transactions: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(txns)
		return 0
	}

	if len(txns) == 0 {
		fmt.Fprintln(stdout, "No transactions found.")
		return 0
	}

	fmt.Fprintf(stdout, "%-36s %-14s %12s %-8s %-30s %s\n",
		"ID", "TYPE", "AMOUNT", "CURRENCY", "DESCRIPTION", "CREATED_AT")
	for _, tx := range txns {
		symbol := currencySymbol(tx.Currency)
		desc := tx.Description
		if len(desc) > 28 {
			desc = desc[:25] + "..."
		}
		createdAt := "-"
		if !tx.CreatedAt.IsZero() {
			createdAt = tx.CreatedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(stdout, "%-36s %-14s %s%-9.2f %-8s %-30s %s\n",
			tx.ID, tx.Type, symbol, tx.Amount, tx.Currency, desc, createdAt)
	}

	if len(txns) >= *limit {
		last := txns[len(txns)-1]
		fmt.Fprintf(stdout, "\nShowing %d results. Use --before %s for more.\n", len(txns), last.ID)
	}
	return 0
}

// ── balance grant ───────────────────────────────────────────────────────────

type grantStatusResponse struct {
	InitialGrant     float64 `json:"initial_grant"`
	PercentRemaining float64 `json:"percent_remaining"`
	Depleted         bool    `json:"depleted"`
}

func runGrant(args []string, tenant string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("balance grant", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	configPath := fs.String("config", "", "config file path")
	storePath := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig(tenant, opts...)
	if err != nil {
		fmt.Fprintf(stderr, "balance grant: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("tenants/%s/grant-status", url.PathEscape(client.TenantID))
	var resp grantStatusResponse
	if err := client.Get(ctx, path, &resp); err != nil {
		fmt.Fprintf(stderr, "balance grant: %v\n", err)
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
	fmt.Fprintf(stdout, "Grant Status (tenant: %s)\n", displayTenant)
	fmt.Fprintf(stdout, "  Initial Grant:  %.2f\n", resp.InitialGrant)
	fmt.Fprintf(stdout, "  Remaining:      %.0f%%\n", resp.PercentRemaining)
	depleted := "no"
	if resp.Depleted {
		depleted = "yes"
	}
	fmt.Fprintf(stdout, "  Depleted:       %s\n", depleted)
	return 0
}

// ── helpers ─────────────────────────────────────────────────────────────────

func buildOpts(configPath, storePath string) []apiclient.Option {
	var opts []apiclient.Option
	if configPath != "" {
		opts = append(opts, apiclient.WithConfigPath(configPath))
	}
	if storePath != "" {
		opts = append(opts, apiclient.WithStorePath(storePath))
	}
	return opts
}

func currencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "CNY":
		return "¥"
	case "USD":
		return "$"
	default:
		return ""
	}
}

func formatAmount(amount float64, currency string) string {
	switch strings.ToUpper(currency) {
	case "CNY", "USD":
		return fmt.Sprintf("%.2f", amount)
	default:
		return fmt.Sprintf("%.4f", amount)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo balance — view tenant balance and transactions

USAGE:
    modelgo balance                    View current balance overview
    modelgo balance transactions       List billing transactions
    modelgo balance grant              View registration grant status

FLAGS:
    --json              Write structured JSON output
    --type TYPE         (transactions) Filter by type: consumption/recharge/refund/grant
    --limit N           (transactions) Number of results, max 100 (default 20)
    --before CURSOR     (transactions) Keyset pagination cursor
    --config PATH       Config file path (default ~/.modelgo/config.json)
    --store PATH        Credential store path (default ~/.modelgo/auth.json)`)
}
