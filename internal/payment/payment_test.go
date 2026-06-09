package payment

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParsePaymentRequired(t *testing.T) {
	body := []byte(`{"x402Version":1,"error":"X-PAYMENT header is required","accepts":[{"scheme":"upto","network":"alipay:cnpc","asset":"CNY","maxAmountRequired":"500","payTo":"m1","resource":"openai.chat_completions"}]}`)
	r, err := ParsePaymentRequired(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.X402Version != 1 || len(r.Accepts) != 1 {
		t.Fatalf("parsed = %+v", r)
	}
	a := r.Accepts[0]
	if a.Scheme != "upto" || a.Network != "alipay:cnpc" || a.MaxAmountRequired != "500" {
		t.Fatalf("accepts[0] = %+v", a)
	}
}

func TestParsePaymentRequired_Invalid(t *testing.T) {
	if _, err := ParsePaymentRequired([]byte(`{"error":"nope"}`)); err == nil {
		t.Fatal("expected error for non-x402 body")
	}
	if _, err := ParsePaymentRequired([]byte(`not json`)); err == nil {
		t.Fatal("expected error for bad json")
	}
}

func TestBuildPaymentHeader(t *testing.T) {
	hdr, err := BuildPaymentHeader("upto", "alipay:cnpc", map[string]any{"credentialToken": "tok", "payerRef": "u9"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(hdr)
	if err != nil {
		t.Fatalf("not base64: %v", err)
	}
	var p paymentPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.X402Version != 1 || p.Scheme != "upto" || p.Network != "alipay:cnpc" || p.Payload["credentialToken"] != "tok" {
		t.Fatalf("payload = %+v", p)
	}
}

func TestBuildPaymentHeader_Errors(t *testing.T) {
	if _, err := BuildPaymentHeader("upto", "", map[string]any{"t": "x"}); err == nil {
		t.Fatal("expected error for empty network")
	}
	if _, err := BuildPaymentHeader("upto", "alipay:cnpc", nil); err == nil {
		t.Fatal("expected error for empty credential")
	}
}

func TestHeaderFromProfile(t *testing.T) {
	hdr, err := HeaderFromProfile(Profile{Network: "alipay:cnpc", Credential: map[string]any{"credentialToken": "t"}})
	if err != nil || hdr == "" {
		t.Fatalf("HeaderFromProfile: %v hdr=%q", err, hdr)
	}
}
