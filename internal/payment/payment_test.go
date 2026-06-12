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

func TestParsePaymentRequiredResponsePrefersV2Header(t *testing.T) {
	body := []byte(`{"x402Version":2,"resource":{"url":"https://gateway.example/v1/chat/completions","description":"chat"},"accepts":[{"scheme":"upto","network":"alipay:cnpc","asset":"CNY","amount":"500","payTo":"merchant_1","maxTimeoutSeconds":60}]}`)
	header := base64.StdEncoding.EncodeToString(body)

	r, rawHeader, err := ParsePaymentRequiredResponse([]byte(`{"error":"stale body"}`), header)
	if err != nil {
		t.Fatalf("ParsePaymentRequiredResponse: %v", err)
	}
	if rawHeader != header {
		t.Fatalf("rawHeader = %q, want original header", rawHeader)
	}
	if r.X402Version != 2 || len(r.Accepts) != 1 {
		t.Fatalf("parsed = %+v", r)
	}
	got := r.Accepts[0]
	if got.Network != "alipay:cnpc" || got.MaxAmountRequired != "500" || got.Resource != "https://gateway.example/v1/chat/completions" || got.Description != "chat" {
		t.Fatalf("accepts[0] = %+v", got)
	}
}

func TestParsePaymentRequiredResponseSynthesizesHeaderForV1Body(t *testing.T) {
	body := []byte(`{"x402Version":1,"accepts":[{"scheme":"upto","network":"alipay:cnpc","asset":"CNY","maxAmountRequired":"600","payTo":"merchant_1","resource":"openai.chat_completions"}]}`)

	r, rawHeader, err := ParsePaymentRequiredResponse(body, "")
	if err != nil {
		t.Fatalf("ParsePaymentRequiredResponse: %v", err)
	}
	if rawHeader != base64.StdEncoding.EncodeToString(body) {
		t.Fatalf("rawHeader = %q, want base64 body", rawHeader)
	}
	if r.Accepts[0].MaxAmountRequired != "600" {
		t.Fatalf("accepts[0] = %+v", r.Accepts[0])
	}
}

func TestSelectAlipayRequirement(t *testing.T) {
	req := &Required{Accepts: []Requirement{
		{Scheme: "exact", Network: "eip155:8453", Asset: "USDC"},
		{Scheme: "upto", Network: "alipay:cnpc", Asset: "CNY"},
	}}
	got, ok := SelectAlipayRequirement(req)
	if !ok {
		t.Fatal("expected alipay requirement")
	}
	if got.Network != "alipay:cnpc" || got.Asset != "CNY" {
		t.Fatalf("requirement = %+v", got)
	}
}

func TestShouldUseAlipaySkill(t *testing.T) {
	if !ShouldUseAlipaySkill("cn", Requirement{Network: "alipay:cnpc"}) {
		t.Fatal("cn + alipay network should use alipay skill")
	}
	if ShouldUseAlipaySkill("intl", Requirement{Network: "alipay:cnpc"}) {
		t.Fatal("intl must not auto-route to alipay skill")
	}
	if ShouldUseAlipaySkill("cn", Requirement{Network: "eip155:8453"}) {
		t.Fatal("non-alipay network must not route to alipay skill")
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
