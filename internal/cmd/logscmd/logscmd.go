// Package logscmd implements `modelgo logs` subcommands: default (list), detail,
// payload, stats, usage. All requests go through the gateway /open/v1/* proxy.
package logscmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/modelgo/modelgo-cli/internal/apiclient"
)

// Run dispatches a `logs` subcommand. args is everything after `modelgo logs`.
// Returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		return runList(args, stdout, stderr)
	}
	switch args[0] {
	case "stats":
		return runStats(args[1:], stdout, stderr)
	case "usage":
		return runUsage(args[1:], stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		// If it looks like a flag, default to list.
		if strings.HasPrefix(args[0], "-") {
			return runList(args, stdout, stderr)
		}
		// Otherwise it's a request-id (detail) or request-id + payload.
		return runDetailOrPayload(args, stdout, stderr)
	}
}

// ── logs (list) ─────────────────────────────────────────────────────────────

type modelLog struct {
	RequestID      string    `json:"request_id"`
	StartedAt      time.Time `json:"started_at"`
	RequestedModel string    `json:"requested_model"`
	Status         string    `json:"status"`
	InputTokens    int       `json:"input_tokens"`
	OutputTokens   int       `json:"output_tokens"`
	TotalTokens    int       `json:"total_tokens"`
	LatencyMs      int       `json:"latency_ms"`
	FinalAmount    float64   `json:"final_amount"`
	Currency       string    `json:"currency"`
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	limit := fs.Int("limit", 20, "number of results (max 100)")
	preset := fs.String("preset", "", "time preset: 1h, 24h, 7d")
	status := fs.String("status", "", "filter by status: success/error/timeout")
	model := fs.String("model", "", "filter by model name")
	workspace := fs.String("workspace", "", "filter by workspace ID")
	apiKey := fs.String("api-key", "", "filter by API key ID")
	configPath := fs.String("config", "", "config file path")
	storePath := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig("", opts...)
	if err != nil {
		fmt.Fprintf(stderr, "logs: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := url.Values{}
	if *limit > 0 && *limit <= 100 {
		params.Set("limit", fmt.Sprintf("%d", *limit))
	}
	if *preset != "" {
		params.Set("preset", *preset)
	}
	if *status != "" {
		params.Set("status", *status)
	}
	if *model != "" {
		params.Set("model", *model)
	}
	if *workspace != "" {
		params.Set("workspace_id", *workspace)
	}
	if *apiKey != "" {
		params.Set("api_key_id", *apiKey)
	}

	var logs []modelLog
	if err := client.GetWithQuery(ctx, "model-logs", params, &logs); err != nil {
		fmt.Fprintf(stderr, "logs: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(logs)
		return 0
	}

	if len(logs) == 0 {
		presetHint := "the last 24h"
		if *preset != "" {
			presetHint = fmt.Sprintf("the last %s", expandPreset(*preset))
		}
		fmt.Fprintf(stdout, "No call logs found in %s. Try --preset 7d for a wider range.\n", presetHint)
		return 0
	}

	fmt.Fprintf(stdout, "%-24s %-20s %-24s %-10s %8s %10s %8s\n",
		"REQUEST_ID", "STARTED_AT", "MODEL", "STATUS", "TOKENS", "LATENCY", "COST")
	for _, l := range logs {
		started := "-"
		if !l.StartedAt.IsZero() {
			started = l.StartedAt.Format("2006-01-02 15:04:05")
		}
		reqID := l.RequestID
		if len(reqID) > 22 {
			reqID = reqID[:22]
		}
		modelName := l.RequestedModel
		if len(modelName) > 22 {
			modelName = modelName[:19] + "..."
		}
		symbol := currencySymbol(l.Currency)
		cost := fmt.Sprintf("%s%.2f", symbol, l.FinalAmount)
		latency := fmt.Sprintf("%dms", l.LatencyMs)
		fmt.Fprintf(stdout, "%-24s %-20s %-24s %-10s %8d %10s %8s\n",
			reqID, started, modelName, l.Status, l.TotalTokens, latency, cost)
	}

	if len(logs) >= *limit {
		fmt.Fprintf(stdout, "\nShowing %d results. Use --limit %d or adjust --preset for more.\n", len(logs), *limit*2)
	}
	return 0
}

// ── logs <request-id> / logs <request-id> payload ──────────────────────────

type modelLogDetail struct {
	RequestID        string    `json:"request_id"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
	RequestedModel   string    `json:"requested_model"`
	Status           string    `json:"status"`
	LatencyMs        int       `json:"latency_ms"`
	TTFTMs           int       `json:"ttft_ms"`
	TPOTMs           int       `json:"tpot_ms"`
	InputTokens      int       `json:"input_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	CacheReadTokens  int       `json:"cache_read_tokens"`
	CacheWriteTokens int       `json:"cache_write_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	FinalAmount      float64   `json:"final_amount"`
	Currency         string    `json:"currency"`
	BillingStatus    string    `json:"billing_status"`
	WorkspaceID      string    `json:"workspace_id"`
	AccountID        string    `json:"account_id"`
	APIKeyID         string    `json:"api_key_id"`
	CallType         string    `json:"call_type"`
	Path             string    `json:"path"`
}

type payloadResponse struct {
	ContentType string `json:"content_type"`
	BodyB64     string `json:"body_b64"`
	Size        int    `json:"size"`
	Truncated   bool   `json:"truncated"`
}

func runDetailOrPayload(args []string, stdout, stderr io.Writer) int {
	requestID := args[0]
	rest := args[1:]

	// Check if next token is "payload"
	if len(rest) > 0 && rest[0] == "payload" {
		return runPayload(requestID, rest[1:], stdout, stderr)
	}

	return runDetail(requestID, rest, stdout, stderr)
}

func runDetail(requestID string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs detail", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	configPath := fs.String("config", "", "config file path")
	storePath := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig("", opts...)
	if err != nil {
		fmt.Fprintf(stderr, "logs: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("model-logs/%s", url.PathEscape(requestID))
	var detail modelLogDetail
	if err := client.Get(ctx, path, &detail); err != nil {
		fmt.Fprintf(stderr, "logs: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(detail)
		return 0
	}

	fmt.Fprintf(stdout, "Call Detail (request: %s)\n", detail.RequestID)
	fmt.Fprintf(stdout, "  Model:           %s\n", detail.RequestedModel)
	fmt.Fprintf(stdout, "  Status:          %s\n", detail.Status)
	started := "-"
	if !detail.StartedAt.IsZero() {
		started = detail.StartedAt.Format("2006-01-02 15:04:05")
	}
	fmt.Fprintf(stdout, "  Started:         %s\n", started)
	completed := "-"
	if !detail.CompletedAt.IsZero() {
		completed = detail.CompletedAt.Format("2006-01-02 15:04:05")
	}
	fmt.Fprintf(stdout, "  Completed:       %s\n", completed)

	latency := fmt.Sprintf("%dms", detail.LatencyMs)
	if detail.TTFTMs > 0 || detail.TPOTMs > 0 {
		latency += fmt.Sprintf(" (TTFT: %dms, TPOT: %dms)", detail.TTFTMs, detail.TPOTMs)
	}
	fmt.Fprintf(stdout, "  Latency:         %s\n", latency)

	fmt.Fprintf(stdout, "  Tokens:          input %s / output %s",
		formatInt(detail.InputTokens), formatInt(detail.OutputTokens))
	if detail.CacheReadTokens > 0 || detail.CacheWriteTokens > 0 {
		fmt.Fprintf(stdout, " / cache_read %s / cache_write %s",
			formatInt(detail.CacheReadTokens), formatInt(detail.CacheWriteTokens))
	}
	fmt.Fprintln(stdout)

	symbol := currencySymbol(detail.Currency)
	fmt.Fprintf(stdout, "  Cost:            %s%.2f\n", symbol, detail.FinalAmount)
	fmt.Fprintf(stdout, "  Billing Status:  %s\n", detail.BillingStatus)
	if detail.WorkspaceID != "" {
		fmt.Fprintf(stdout, "  Workspace:       %s\n", detail.WorkspaceID)
	}
	if detail.APIKeyID != "" {
		// Mask the API key for security.
		masked := maskAPIKey(detail.APIKeyID)
		fmt.Fprintf(stdout, "  API Key:         %s\n", masked)
	}
	if detail.CallType != "" {
		fmt.Fprintf(stdout, "  Call Type:       %s\n", detail.CallType)
	}
	if detail.Path != "" {
		fmt.Fprintf(stdout, "  Path:            %s\n", detail.Path)
	}
	return 0
}

func runPayload(requestID string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs payload", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	kind := fs.String("kind", "response", "payload kind: request or response")
	configPath := fs.String("config", "", "config file path")
	storePath := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig("", opts...)
	if err != nil {
		fmt.Fprintf(stderr, "logs payload: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("model-logs/%s/payload", url.PathEscape(requestID))
	params := url.Values{}
	if *kind != "" {
		params.Set("kind", *kind)
	}
	var resp payloadResponse
	if err := client.GetWithQuery(ctx, path, params, &resp); err != nil {
		fmt.Fprintf(stderr, "logs payload: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(resp)
		return 0
	}

	if resp.BodyB64 == "" {
		fmt.Fprintf(stdout, "No %s payload available.\n", *kind)
		return 0
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.BodyB64)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(resp.BodyB64)
		if err != nil {
			fmt.Fprintf(stderr, "logs payload: failed to decode base64: %v\n", err)
			return 1
		}
	}

	// Try to pretty-print JSON.
	var jsonObj any
	if json.Unmarshal(decoded, &jsonObj) == nil {
		pretty, err := json.MarshalIndent(jsonObj, "", "  ")
		if err == nil {
			decoded = pretty
		}
	}
	fmt.Fprintf(stdout, "Payload (%s, %s, %d bytes):\n", requestID, *kind, resp.Size)
	fmt.Fprintln(stdout, string(decoded))
	if resp.Truncated {
		fmt.Fprintf(stdout, "\n... (truncated, total size: %d bytes)\n", resp.Size)
	}
	return 0
}

// ── logs stats ──────────────────────────────────────────────────────────────

type statsGroup struct {
	Model        string  `json:"model"`
	Requests     int     `json:"requests"`
	Errors       int     `json:"errors"`
	ErrorRate    float64 `json:"error_rate"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	AvgLatencyMs float64 `json:"average_latency_ms"`
	Cost         float64 `json:"cost"`
	Currency     string  `json:"currency"`
}

type statsResponse struct {
	Groups []statsGroup `json:"groups"`
}

func runStats(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs stats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	from := fs.String("from", "", "start date (YYYY-MM-DD)")
	to := fs.String("to", "", "end date (YYYY-MM-DD)")
	model := fs.String("model", "", "filter by model name")
	workspace := fs.String("workspace", "", "filter by workspace ID")
	groupBy := fs.String("group-by", "model", "group by: none/model/provider/workspace/creator/api_key")
	granularity := fs.String("granularity", "day", "time granularity: hour/day")
	configPath := fs.String("config", "", "config file path")
	storePath := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig("", opts...)
	if err != nil {
		fmt.Fprintf(stderr, "logs stats: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := url.Values{}
	if *from != "" {
		params.Set("from", *from)
	}
	if *to != "" {
		params.Set("to", *to)
	}
	if *model != "" {
		params.Set("model", *model)
	}
	if *workspace != "" {
		params.Set("workspace_id", *workspace)
	}
	if *groupBy != "" {
		params.Set("group_by", *groupBy)
	}
	if *granularity != "" {
		params.Set("granularity", *granularity)
	}

	var resp statsResponse
	if err := client.GetWithQuery(ctx, "model-logs/stats", params, &resp); err != nil {
		fmt.Fprintf(stderr, "logs stats: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(resp)
		return 0
	}

	fmt.Fprintf(stdout, "Call Stats (group by: %s, granularity: %s)\n\n", *groupBy, *granularity)
	if len(resp.Groups) == 0 {
		fmt.Fprintln(stdout, "No data available for the selected period.")
		return 0
	}

	for _, g := range resp.Groups {
		symbol := currencySymbol(g.Currency)
		label := g.Model
		if label == "" {
			label = "(ungrouped)"
		}
		fmt.Fprintf(stdout, "%s\n", label)
		fmt.Fprintf(stdout, "  Requests:   %s", formatInt64(int64(g.Requests)))
		if g.Errors > 0 {
			fmt.Fprintf(stdout, "    Errors: %d (%.2f%%)", g.Errors, g.ErrorRate*100)
		}
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "  Tokens:     in %s / out %s\n",
			formatInt64(g.InputTokens), formatInt64(g.OutputTokens))
		fmt.Fprintf(stdout, "  Avg Latency: %.0fms\n", g.AvgLatencyMs)
		fmt.Fprintf(stdout, "  Cost:       %s%.2f\n", symbol, g.Cost)
		fmt.Fprintln(stdout)
	}
	return 0
}

// ── logs usage ──────────────────────────────────────────────────────────────

type usageTotal struct {
	Spend          float64 `json:"spend"`
	Currency       string  `json:"currency"`
	Requests       int64   `json:"requests"`
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	ErrorRate      float64 `json:"error_rate"`
	AverageLatency float64 `json:"average_latency_ms"`
}

type usageResponse struct {
	Period string     `json:"period"`
	Total  usageTotal `json:"total"`
}

func runUsage(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs usage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	from := fs.String("from", "", "start date (YYYY-MM-DD)")
	to := fs.String("to", "", "end date (YYYY-MM-DD)")
	configPath := fs.String("config", "", "config file path")
	storePath := fs.String("store", "", "credential store path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := buildOpts(*configPath, *storePath)
	client, err := apiclient.NewFromConfig("", opts...)
	if err != nil {
		fmt.Fprintf(stderr, "logs usage: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := url.Values{}
	if *from != "" {
		params.Set("from", *from)
	}
	if *to != "" {
		params.Set("to", *to)
	}

	var resp usageResponse
	if err := client.GetWithQuery(ctx, "usage/summary", params, &resp); err != nil {
		fmt.Fprintf(stderr, "logs usage: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(resp)
		return 0
	}

	fmt.Fprintln(stdout, "Usage Summary")
	fmt.Fprintln(stdout)
	if resp.Period != "" {
		fmt.Fprintf(stdout, "Period:  %s\n", resp.Period)
	}
	symbol := currencySymbol(resp.Total.Currency)
	fmt.Fprintf(stdout, "Spend:   %s%.2f\n", symbol, resp.Total.Spend)
	fmt.Fprintf(stdout, "Requests: %s\n", formatInt64(resp.Total.Requests))
	fmt.Fprintf(stdout, "Tokens:  in %s / out %s\n",
		formatInt64(resp.Total.InputTokens), formatInt64(resp.Total.OutputTokens))
	fmt.Fprintf(stdout, "Errors:  %.2f%%\n", resp.Total.ErrorRate*100)
	fmt.Fprintf(stdout, "Avg Latency: %.0fms\n", resp.Total.AverageLatency)
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

func formatInt(n int) string {
	return formatInt64(int64(n))
}

func formatInt64(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// Simple thousands separator.
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func maskAPIKey(id string) string {
	if len(id) <= 8 {
		return "***"
	}
	return id[:4] + "***" + id[len(id)-4:]
}

func expandPreset(p string) string {
	switch p {
	case "1h":
		return "hour"
	case "24h":
		return "24 hours"
	case "7d":
		return "7 days"
	default:
		return p
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo logs — query call logs and usage statistics

USAGE:
    modelgo logs                         List recent call logs
    modelgo logs <request-id>            View call detail
    modelgo logs <request-id> payload    View request/response payload
    modelgo logs stats                   Call statistics by group
    modelgo logs usage                   Usage summary

FLAGS:
    --json                 Write structured JSON output
    --limit N              (list) Number of results, max 100 (default 20)
    --preset DURATION      (list) Time preset: 1h, 24h, 7d
    --status STATUS        (list) Filter by status: success/error/timeout
    --model MODEL          (list/stats) Filter by model name
    --workspace ID         (list/stats) Filter by workspace ID
    --api-key ID           (list) Filter by API key ID
    --kind KIND            (payload) Payload kind: request or response (default response)
    --from DATE            (stats/usage) Start date (YYYY-MM-DD)
    --to DATE              (stats/usage) End date (YYYY-MM-DD)
    --group-by DIM         (stats) Group by: none/model/provider/workspace/creator/api_key (default model)
    --granularity G        (stats) Time granularity: hour/day (default day)
    --config PATH          Config file path (default ~/.modelgo/config.json)
    --store PATH           Credential store path (default ~/.modelgo/auth.json)`)
}
