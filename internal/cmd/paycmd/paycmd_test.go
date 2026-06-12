package paycmd

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestCNAlipay402WritesHandoff(t *testing.T) {
	t.Parallel()
	var sawProtocol, sawNetwork, sawAnonymousID, sawBody string
	requiredBody := `{"x402Version":2,"resource":{"url":"https://gateway.example/v1/chat/completions","description":"OpenAI chat completion"},"accepts":[{"scheme":"upto","network":"alipay:cnpc","asset":"CNY","amount":"500","payTo":"merchant_1","maxTimeoutSeconds":60}]}`
	requiredHeader := base64.StdEncoding.EncodeToString([]byte(requiredBody))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawProtocol = r.Header.Get("X-Payment-Protocol")
		sawNetwork = r.Header.Get("X-Payment-Network")
		sawAnonymousID = r.Header.Get("X-ModelGo-Anonymous-ID")
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		sawBody = string(body)
		w.Header().Set("PAYMENT-REQUIRED", requiredHeader)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(requiredBody))
	}))
	defer server.Close()

	paymentDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	var stdout, stderr strings.Builder
	code := Run([]string{
		"request",
		"--env", "cn",
		"--config", configPath,
		"--url", server.URL + "/v1/chat/completions",
		"--method", "POST",
		"--data", `{"model":"gpt-4o","messages":[]}`,
		"--intent", "调用 gpt-4o 完成聊天补全",
		"--payment-dir", paymentDir,
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run code = %d stderr=%s", code, stderr.String())
	}
	if sawProtocol != "x402" || sawNetwork != "alipay:cnpc" {
		t.Fatalf("request headers protocol=%q network=%q", sawProtocol, sawNetwork)
	}
	if sawAnonymousID == "" || !strings.HasPrefix(sawAnonymousID, "anon_") {
		t.Fatalf("anonymous id header = %q", sawAnonymousID)
	}
	if sawBody != `{"model":"gpt-4o","messages":[]}` {
		t.Fatalf("body = %q", sawBody)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &out); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if out["event"] != "x402_payment_required" || out["payment_skill"] != "alipay-payment-skill" {
		t.Fatalf("handoff = %+v", out)
	}
	if out["intent_summary"] != "原始请求：调用 gpt-4o 完成聊天补全" {
		t.Fatalf("intent_summary = %v", out["intent_summary"])
	}
	fileName, _ := out["payment_required_file"].(string)
	if fileName == "" {
		t.Fatalf("missing payment_required_file: %+v", out)
	}
	if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		t.Fatalf("payment_required_file must be a safe basename, got %q", fileName)
	}
	got, err := os.ReadFile(filepath.Join(paymentDir, fileName))
	if err != nil {
		t.Fatalf("read payment file: %v", err)
	}
	if string(got) != requiredHeader {
		t.Fatalf("payment file = %q, want header", string(got))
	}
	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(cfgBytes), sawAnonymousID) {
		t.Fatalf("config did not persist anonymous id %q: %s", sawAnonymousID, string(cfgBytes))
	}
}

func TestRequestIntlDoesNotRouteToAlipaySkill(t *testing.T) {
	t.Parallel()
	requiredBody := `{"x402Version":2,"resource":{"url":"https://gateway.example/v1/chat/completions"},"accepts":[{"scheme":"upto","network":"alipay:cnpc","asset":"CNY","amount":"500","payTo":"merchant_1"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("PAYMENT-REQUIRED", base64.StdEncoding.EncodeToString([]byte(requiredBody)))
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(requiredBody))
	}))
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{
		"request",
		"--env", "intl",
		"--url", server.URL + "/v1/chat/completions",
		"--payment-dir", t.TempDir(),
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run code = %d stderr=%s", code, stderr.String())
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &out); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if _, ok := out["payment_skill"]; ok {
		t.Fatalf("intl response must not include alipay skill handoff: %+v", out)
	}
	if out["next_action"] != "configure_non_alipay_x402_payment" {
		t.Fatalf("next_action = %v", out["next_action"])
	}
}

func TestRequestSuccessPrintsResourceBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Payment-Protocol") != "x402" {
			t.Fatalf("missing x402 header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1"}`))
	}))
	defer server.Close()

	var stdout, stderr strings.Builder
	code := Run([]string{"request", "--env", "cn", "--url", server.URL + "/v1/chat/completions"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run code = %d stderr=%s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != `{"id":"chatcmpl_1"}` {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
