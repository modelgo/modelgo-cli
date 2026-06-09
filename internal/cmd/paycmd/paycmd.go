// Package paycmd implements `modelgo pay`: manage the x402 (pay-per-call)
// payment profile the CLI / an AI agent uses to satisfy a gateway 402.
//
// Scope: profile + credential management and header construction. Acquiring a
// real Alipay AI-Collect credential (and a `modelgo chat` command that triggers
// a 402 and auto-retries) are TODOs — see runSet.
package paycmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/modelgo/modelgo-cli/internal/config"
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

Examples:
  modelgo pay set --method alipay --network alipay:cnpc --token <agent_token>
  modelgo pay header --json
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
