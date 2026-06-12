// Package paycmd implements `modelgo pay`: manage the x402 (pay-per-call)
// payment profile the CLI / an AI agent uses to satisfy a gateway 402.
//
// Scope: profile + credential management and header construction. Acquiring a
// real Alipay AI-Collect credential (and a `modelgo chat` command that triggers
// a 402 and auto-retries) are TODOs — see runSet.
package paycmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelgo/modelgo-cli/internal/config"
	clienv "github.com/modelgo/modelgo-cli/internal/env"
	"github.com/modelgo/modelgo-cli/internal/payment"
)

// Run dispatches a `pay` subcommand. Returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "methods":
		return runMethods(args[1:], stdout, stderr)
	case "set":
		return runSet(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "header":
		return runHeader(args[1:], stdout, stderr)
	case "request":
		return runRequest(args[1:], stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown pay subcommand: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: modelgo pay <subcommand>

Manage the x402 (pay-per-call) payment profile used to satisfy a gateway 402.

Subcommands:
  methods            List the payment channels the gateway can advertise.
  set                Store a payment profile (network/scheme/credential).
  status             Show the stored payment profile (credential redacted).
  header             Print the X-Payment-Protocol + X-PAYMENT headers to attach
                     to a request (for an agent / manual retry).
  request            Call a model API with x402 enabled; on domestic 402,
                     prepare an Alipay skill handoff.

Examples:
  modelgo pay set --method alipay --network alipay:cnpc --token <agent_token>
  modelgo pay header --json
  modelgo pay request --path /v1/chat/completions --method POST --data '{"model":"gpt-4o","messages":[]}'
`)
}

func runMethods(args []string, stdout, stderr io.Writer) int {
	var asJSON bool
	fs := flag.NewFlagSet("pay methods", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&asJSON, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	methods := []map[string]string{
		{"method": "alipay", "network": "alipay:cnpc", "scheme": "upto", "asset": "CNY", "status": "available"},
		{"method": "blockchain", "network": "eip155:*", "scheme": "exact", "asset": "USDC", "status": "planned"},
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(methods)
		return 0
	}
	fmt.Fprintln(stdout, "Payment channels the gateway can advertise in a 402:")
	for _, m := range methods {
		fmt.Fprintf(stdout, "  - %-10s network=%-12s scheme=%-6s asset=%-5s (%s)\n",
			m["method"], m["network"], m["scheme"], m["asset"], m["status"])
	}
	fmt.Fprintln(stdout, "\nThe gateway's 402 response lists the live accepts[]; store one with `modelgo pay set`.")
	return 0
}

func runSet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pay set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var method, network, scheme, token, payerRef string
	fs.StringVar(&method, "method", "alipay", "payment method: alipay | blockchain")
	fs.StringVar(&network, "network", "alipay:cnpc", "CAIP-2 network")
	fs.StringVar(&scheme, "scheme", "upto", "x402 scheme")
	fs.StringVar(&token, "token", "", "agent payment credential token")
	fs.StringVar(&payerRef, "payer", "", "optional payer reference")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if token == "" {
		// TODO(x402-alipay): acquire the credential interactively (open the
		// Alipay AI-Collect checkout / authorize flow) instead of requiring a
		// pre-obtained --token. For now the token must be supplied.
		fmt.Fprintln(stderr, "error: --token is required (interactive Alipay acquisition is not yet implemented)")
		return 2
	}
	cred := map[string]any{"credentialToken": token}
	if payerRef != "" {
		cred["payerRef"] = payerRef
	}

	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: load config: %v\n", err)
		return 1
	}
	cfg.Payment = &config.PaymentProfile{Method: method, Network: network, Scheme: scheme, Credential: cred}
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintf(stderr, "error: save config: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "✓ payment profile saved: method=%s network=%s scheme=%s\n", method, network, scheme)
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	var asJSON bool
	fs := flag.NewFlagSet("pay status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&asJSON, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: load config: %v\n", err)
		return 1
	}
	if cfg.Payment == nil {
		if asJSON {
			fmt.Fprintln(stdout, "{}")
		} else {
			fmt.Fprintln(stdout, "no payment profile set. Run: modelgo pay set --token <token>")
		}
		return 0
	}
	p := cfg.Payment
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"method": p.Method, "network": p.Network, "scheme": p.Scheme,
			"credential_set": len(p.Credential) > 0,
		})
		return 0
	}
	fmt.Fprintf(stdout, "method:     %s\nnetwork:    %s\nscheme:     %s\ncredential: %s\n",
		p.Method, p.Network, p.Scheme, redacted(len(p.Credential) > 0))
	return 0
}

func runHeader(args []string, stdout, stderr io.Writer) int {
	var asJSON bool
	fs := flag.NewFlagSet("pay header", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&asJSON, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: load config: %v\n", err)
		return 1
	}
	if cfg.Payment == nil {
		fmt.Fprintln(stderr, "error: no payment profile set. Run: modelgo pay set --token <token>")
		return 1
	}
	hdr, err := payment.HeaderFromProfile(payment.Profile{
		Method:     cfg.Payment.Method,
		Network:    cfg.Payment.Network,
		Scheme:     cfg.Payment.Scheme,
		Credential: cfg.Payment.Credential,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: build header: %v\n", err)
		return 1
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]string{
			payment.HeaderProtocol: payment.ProtocolValue,
			payment.HeaderNetwork:  cfg.Payment.Network,
			payment.HeaderPayment:  hdr,
		})
		return 0
	}
	// Headers to attach to a model request to pay via x402.
	fmt.Fprintf(stdout, "%s: %s\n%s: %s\n%s: %s\n",
		payment.HeaderProtocol, payment.ProtocolValue,
		payment.HeaderNetwork, cfg.Payment.Network,
		payment.HeaderPayment, hdr)
	return 0
}

func redacted(set bool) string {
	if set {
		return "(set)"
	}
	return "(none)"
}

type headerList []string

func (h *headerList) String() string { return strings.Join(*h, ",") }
func (h *headerList) Set(v string) error {
	if !strings.Contains(v, ":") {
		return fmt.Errorf("header must be KEY:VALUE")
	}
	*h = append(*h, v)
	return nil
}

func runRequest(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pay request", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to use (default: active env from config)")
	configPath := fs.String("config", "", "config file path (default ~/.modelgo/config.json)")
	rawURL := fs.String("url", "", "absolute model API URL")
	path := fs.String("path", "", "model API path relative to the env base URL, e.g. /v1/chat/completions")
	method := fs.String("method", http.MethodGet, "HTTP method")
	data := fs.String("data", "", "request body")
	dataFile := fs.String("data-file", "", "file containing request body")
	network := fs.String("network", "", "preferred x402 network")
	intent := fs.String("intent", "", "original user request summary for payment handoff")
	paymentDir := fs.String("payment-dir", ".", "directory for the generated PAYMENT-REQUIRED file")
	asJSON := fs.Bool("json", false, "output JSON for agents")
	var headers headerList
	fs.Var(&headers, "header", "additional request header KEY:VALUE; repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "pay request: unexpected argument %q\n", fs.Arg(0))
		return 2
	}

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: load config: %v\n", err)
		return 1
	}
	anonymousID, err := ensureAnonymousID(cfgPath, &cfg)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: anonymous id: %v\n", err)
		return 1
	}
	envName := clienv.ActiveEnv(*envFlag, cfg)
	baseURL, err := clienv.Resolve(envName, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: %v\n", err)
		return 1
	}

	targetURL, err := resolveRequestURL(baseURL, *rawURL, *path)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: %v\n", err)
		return 2
	}
	body, err := requestBody(*data, *dataFile)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: %v\n", err)
		return 1
	}
	reqMethod := strings.ToUpper(strings.TrimSpace(*method))
	if reqMethod == "" {
		reqMethod = http.MethodGet
	}
	if body != nil && reqMethod == http.MethodGet {
		reqMethod = http.MethodPost
	}
	preferredNetwork := strings.TrimSpace(*network)
	if preferredNetwork == "" && envName == "cn" {
		preferredNetwork = "alipay:cnpc"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, reqMethod, targetURL, body)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: build request: %v\n", err)
		return 1
	}
	req.Header.Set(payment.HeaderProtocol, payment.ProtocolValue)
	req.Header.Set("X-ModelGo-Anonymous-ID", anonymousID)
	if preferredNetwork != "" {
		req.Header.Set(payment.HeaderNetwork, preferredNetwork)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, h := range headers {
		key, value, _ := strings.Cut(h, ":")
		req.Header.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "pay request: read response: %v\n", err)
		return 1
	}
	if resp.StatusCode != http.StatusPaymentRequired {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			fmt.Fprintf(stderr, "pay request: HTTP %d\n%s\n", resp.StatusCode, string(respBody))
			return 1
		}
		_, _ = stdout.Write(respBody)
		if len(respBody) == 0 || respBody[len(respBody)-1] != '\n' {
			fmt.Fprintln(stdout)
		}
		return 0
	}

	required, rawPaymentRequired, err := payment.ParsePaymentRequiredResponse(respBody, resp.Header.Get(payment.HeaderRequired))
	if err != nil {
		fmt.Fprintf(stderr, "pay request: parse x402 402: %v\n", err)
		return 1
	}
	alipayReq, hasAlipay := payment.SelectAlipayRequirement(required)
	if hasAlipay && payment.ShouldUseAlipaySkill(envName, alipayReq) {
		fileName, err := writePaymentRequiredFile(*paymentDir, rawPaymentRequired)
		if err != nil {
			fmt.Fprintf(stderr, "pay request: write payment requirements: %v\n", err)
			return 1
		}
		out := map[string]any{
			"event":                 "x402_payment_required",
			"env":                   envName,
			"payment_skill":         "alipay-payment-skill",
			"payment_required_file": fileName,
			"working_directory":     cleanDir(*paymentDir),
			"resource_url":          targetURL,
			"method":                reqMethod,
			"data":                  stringFromBytes(bodyBytes(*data, *dataFile)),
			"headers":               replayHeaders(headers, body != nil),
			"intent_summary":        intentSummary(*intent, reqMethod, targetURL),
			"requirement":           alipayReq,
			"next_action":           "load_alipay_payment_skill_and_run_402_buyer_pay",
		}
		if *asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(out)
			return 0
		}
		printAlipayHandoff(stdout, out)
		return 0
	}

	out := map[string]any{
		"event":        "x402_payment_required",
		"env":          envName,
		"resource_url": targetURL,
		"accepts":      required.Accepts,
		"next_action":  "configure_non_alipay_x402_payment",
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 0
	}
	fmt.Fprintln(stdout, "HTTP 402 Payment Required.")
	fmt.Fprintln(stdout, "This environment is not routed to Alipay. Configure a non-Alipay x402 payment profile and retry.")
	return 0
}

func resolveRequestURL(baseURL, rawURL, path string) (string, error) {
	if rawURL != "" {
		u, err := url.Parse(rawURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", fmt.Errorf("--url must be absolute")
		}
		return rawURL, nil
	}
	if path == "" {
		return "", fmt.Errorf("one of --url or --path is required")
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid resolved URL")
	}
	return u.String(), nil
}

func requestBody(data, file string) (io.Reader, error) {
	b, err := bodyBytes(data, file)
	if err != nil || b == nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

func bodyBytes(data, file string) ([]byte, error) {
	if data != "" && file != "" {
		return nil, fmt.Errorf("--data and --data-file are mutually exclusive")
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	if data == "" {
		return nil, nil
	}
	return []byte(data), nil
}

func stringFromBytes(b []byte, err error) string {
	if err != nil || b == nil {
		return ""
	}
	return string(b)
}

func writePaymentRequiredFile(dir, content string) (string, error) {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := fmt.Sprintf("402_required_%d.txt", time.Now().UnixNano())
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		return "", err
	}
	return name, nil
}

func cleanDir(dir string) string {
	if dir == "" {
		return "."
	}
	return filepath.Clean(dir)
}

func replayHeaders(headers []string, hasBody bool) []string {
	out := make([]string, 0, len(headers)+1)
	if hasBody && !hasHeader(headers, "Content-Type") {
		out = append(out, "Content-Type:application/json")
	}
	for _, h := range headers {
		key, value, _ := strings.Cut(h, ":")
		out = append(out, strings.TrimSpace(key)+":"+strings.TrimSpace(value))
	}
	return out
}

func hasHeader(headers []string, name string) bool {
	for _, h := range headers {
		key, _, _ := strings.Cut(h, ":")
		if strings.EqualFold(strings.TrimSpace(key), name) {
			return true
		}
	}
	return false
}

func intentSummary(intent, method, targetURL string) string {
	intent = strings.TrimSpace(intent)
	if intent == "" {
		intent = method + " " + targetURL
	}
	return "原始请求：" + intent
}

func printAlipayHandoff(w io.Writer, out map[string]any) {
	fmt.Fprintln(w, "HTTP 402 Payment Required.")
	fmt.Fprintln(w, "国内环境已生成支付宝 x402 支付交接信息。请加载支付宝支付处理技能 alipay-payment-skill，并按以下字段执行 402-buyer-pay。")
	fmt.Fprintf(w, "working_directory: %s\n", out["working_directory"])
	fmt.Fprintf(w, "payment_required_file: %s\n", out["payment_required_file"])
	fmt.Fprintf(w, "resource_url: %s\n", out["resource_url"])
	fmt.Fprintf(w, "method: %s\n", out["method"])
	if data, _ := out["data"].(string); data != "" {
		fmt.Fprintf(w, "data: %s\n", data)
	}
	if headers, ok := out["headers"].([]string); ok && len(headers) > 0 {
		fmt.Fprintf(w, "headers: %s\n", strings.Join(headers, " | "))
	}
	fmt.Fprintf(w, "intent_summary: %s\n", out["intent_summary"])
}

func ensureAnonymousID(path string, cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("missing config")
	}
	if cfg.AnonymousID != "" {
		return cfg.AnonymousID, nil
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	cfg.AnonymousID = "anon_" + hex.EncodeToString(b[:])
	return cfg.AnonymousID, config.Save(path, *cfg)
}
