package logscmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/auth"
	"github.com/modelgo/modelgo-cli/internal/config"
)

func setupTestEnv(t *testing.T, srvURL string) (configPath, storePath string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	authPath := filepath.Join(dir, "auth.json")

	cfg := config.Config{
		CurrentEnv: "cn",
		Envs: map[string]config.EnvEntry{
			"cn": {BaseURL: srvURL},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0o644)

	cred := auth.Credential{
		Env: "cn", BaseURL: srvURL, SessionToken: "test-token",
		TenantID: "ten_test123", TenantSlug: "acme", TokenType: "Session",
	}
	auth.SaveCredential(authPath, cred)

	return cfgPath, authPath
}

func TestRunList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/model-logs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "20" {
			t.Errorf("limit = %q, want 20", r.URL.Query().Get("limit"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": []map[string]any{
				{
					"request_id":      "abc123",
					"requested_model": "gpt-4o",
					"status":          "success",
					"total_tokens":    1523,
					"latency_ms":      1230,
					"final_amount":    0.15,
					"currency":        "CNY",
				},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", cfgPath, "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("abc123")) {
		t.Errorf("expected request_id in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("gpt-4o")) {
		t.Errorf("expected model name in output, got: %s", out)
	}
}

func TestRunDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/model-logs/req-abc" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"request_id":      "req-abc",
				"requested_model": "gpt-4o",
				"status":          "success",
				"latency_ms":      1230,
				"ttft_ms":         320,
				"tpot_ms":         45,
				"input_tokens":    1024,
				"output_tokens":   499,
				"final_amount":    0.15,
				"currency":        "CNY",
				"billing_status":  "settled",
				"call_type":       "chat",
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"req-abc", "--config", cfgPath, "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("TTFT")) {
		t.Errorf("expected TTFT in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("settled")) {
		t.Errorf("expected billing status in output, got: %s", out)
	}
}

func TestRunStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/model-logs/stats" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"groups": []map[string]any{
					{
						"model":              "gpt-4o",
						"requests":           1234,
						"errors":             12,
						"error_rate":         0.0097,
						"input_tokens":       523000,
						"output_tokens":      89000,
						"average_latency_ms": 1450,
						"cost":               123.45,
						"currency":           "CNY",
					},
				},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "--config", cfgPath, "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("gpt-4o")) {
		t.Errorf("expected model name in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("1,234")) {
		t.Errorf("expected formatted request count, got: %s", out)
	}
}

func TestRunUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/usage/summary" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"period": "2026-05-28 ~ 2026-06-04",
				"total": map[string]any{
					"spend":              191.34,
					"currency":           "CNY",
					"requests":           1801,
					"input_tokens":       757000,
					"output_tokens":      134000,
					"error_rate":         0.0083,
					"average_latency_ms": 1215,
				},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"usage", "--config", cfgPath, "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("Usage Summary")) {
		t.Errorf("expected header in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("1,801")) {
		t.Errorf("expected formatted request count, got: %s", out)
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{12, "12"},
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := formatInt64(tt.in)
		if got != tt.want {
			t.Errorf("formatInt64(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"mgk_prod_abc12345", "mgk_***2345"},
		{"short", "***"},
	}
	for _, tt := range tests {
		got := maskAPIKey(tt.in)
		if got != tt.want {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
