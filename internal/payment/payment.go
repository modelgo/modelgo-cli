// Package payment provides the client-side x402 (pay-per-call) primitives the
// CLI uses to cooperate with the model-gateway: parsing a 402 "payment
// required" response, building the X-PAYMENT header from a stored credential,
// and the dispatch headers that opt a request into the x402 path.
//
// The CLI does not (yet) make model calls itself; these primitives are the
// reusable seam a future `modelgo chat`-style command — or an AI agent driving
// the CLI — uses to complete the 402 → pay → retry loop. Acquiring a real
// Alipay AI-Collect credential (the contents of Profile.Credential) is a
// separate, channel-specific step — see acquireAlipayCredential's TODO.
package payment

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Dispatch + credential headers (must match modelgo-model-gateway pkg/x402).
const (
	HeaderProtocol = "X-Payment-Protocol" // marker: opt into the x402 path
	HeaderNetwork  = "X-Payment-Network"  // optional preferred network
	HeaderPayment  = "X-PAYMENT"          // v1 credential header
	ProtocolValue  = "x402"
)

// Requirement is one accepts[] entry from a 402 response (v1 field names).
type Requirement struct {
	Scheme            string `json:"scheme"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
	MaxAmountRequired string `json:"maxAmountRequired"`
	PayTo             string `json:"payTo"`
	Resource          string `json:"resource"`
	Description       string `json:"description"`
}

// Required is a decoded 402 body.
type Required struct {
	X402Version int           `json:"x402Version"`
	Error       string        `json:"error"`
	Accepts     []Requirement `json:"accepts"`
}

// ParsePaymentRequired decodes a 402 response body into its payment
// requirements. Returns an error if the body is not a recognizable x402 402.
func ParsePaymentRequired(body []byte) (*Required, error) {
	var r Required
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("payment: parse 402 body: %w", err)
	}
	if r.X402Version == 0 || len(r.Accepts) == 0 {
		return nil, fmt.Errorf("payment: response is not a valid x402 402 (version=%d, accepts=%d)", r.X402Version, len(r.Accepts))
	}
	return &r, nil
}

// Profile is the stored payment preference + credential (persisted in the CLI
// config). Credential is the channel-specific agent credential forwarded
// verbatim in the X-PAYMENT payload (e.g. Alipay token / nonce / signature).
type Profile struct {
	Method     string         `json:"method,omitempty"`  // "alipay" | "blockchain"
	Network    string         `json:"network,omitempty"` // CAIP-2, e.g. "alipay:cnpc"
	Scheme     string         `json:"scheme,omitempty"`  // defaults to "upto"
	Credential map[string]any `json:"credential,omitempty"`
}

// paymentPayload is the X-PAYMENT body (v1 wire shape).
type paymentPayload struct {
	X402Version int            `json:"x402Version"`
	Scheme      string         `json:"scheme"`
	Network     string         `json:"network"`
	Payload     map[string]any `json:"payload"`
}

// BuildPaymentHeader builds the base64 X-PAYMENT header value for a v1 payment
// from the given scheme/network and credential payload.
func BuildPaymentHeader(scheme, network string, credential map[string]any) (string, error) {
	if scheme == "" {
		scheme = "upto"
	}
	if network == "" {
		return "", fmt.Errorf("payment: network is required")
	}
	if len(credential) == 0 {
		return "", fmt.Errorf("payment: empty credential")
	}
	raw, err := json.Marshal(paymentPayload{
		X402Version: 1,
		Scheme:      scheme,
		Network:     network,
		Payload:     credential,
	})
	if err != nil {
		return "", fmt.Errorf("payment: marshal payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// HeaderFromProfile builds the X-PAYMENT header from a stored Profile.
func HeaderFromProfile(p Profile) (string, error) {
	return BuildPaymentHeader(p.Scheme, p.Network, p.Credential)
}
