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
			"data": map[string]any{ // observer list wrapper: {items, limit}
				"limit": 20,
				"items": []map[string]any{
					{
						"request_id":      "abc123",
						"requested_model": "gpt-4o",
						"status":          "success",
						"total_tokens":    1523,
						"latency_ms":      1230,
						"final_amount":    "0.15", // string-encoded decimal upstream
						"currency":        "CNY",
					},
				},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
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
			"data": map[string]any{ // observer detail wrapper: {log: {row}}
				"log": map[string]any{
					"request_id":      "req-abc",
					"requested_model": "gpt-4o",
					"status":          "success",
					"latency_ms":      1230,
					"ttft_ms":         320,
					"tpot_ms":         45,
					"input_tokens":    1024,
					"output_tokens":   499,
					"final_amount":    "0.15", // string-encoded decimal upstream
					"currency":        "CNY",
					"billing_status":  "settled",
					"call_type":       "chat",
				},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"req-abc", "--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
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
			"data": map[string]any{ // observer stats: {totals, groups:[{key,label,spend,requests,tokens}]}
				"from": "2026-06-01T00:00:00Z", "to": "2026-06-13T00:00:00Z",
				"granularity": "day", "group_by": "model",
				"totals": map[string]any{"spend": "123.45", "requests": 1234, "tokens": 612000},
				"groups": []map[string]any{
					{"key": "gpt-4o", "label": "gpt-4o", "spend": "123.45", "requests": 1234, "tokens": 612000},
				},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
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

// observer's model-logs/stats validates from/to as RFC3339 and 400s on a bare
// YYYY-MM-DD. The CLI documents --from/--to as YYYY-MM-DD, so it must widen a
// bare date to the start-of-day RFC3339 timestamp before sending. (Regression
// for a contract bug found via live gateway e2e: bare dates → HTTP 400.)
func TestRunStats_NormalizesBareDateToRFC3339(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("from"); got != "2026-06-06T00:00:00Z" {
			t.Errorf("from = %q, want 2026-06-06T00:00:00Z (RFC3339)", got)
		}
		if got := r.URL.Query().Get("to"); got != "2026-06-14T00:00:00Z" {
			t.Errorf("to = %q, want 2026-06-14T00:00:00Z (RFC3339)", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"totals": map[string]any{"spend": "1", "requests": 1, "tokens": 1},
				"groups": []map[string]any{},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "--from", "2026-06-06", "--to", "2026-06-14", "--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
}

func TestNormalizeDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"2026-06-06", "2026-06-06T00:00:00Z"},
		{"  2026-06-06  ", "2026-06-06T00:00:00Z"},
		{"2026-06-06T08:30:00Z", "2026-06-06T08:30:00Z"}, // already RFC3339: pass through
		{"not-a-date", "not-a-date"},                     // let the server validate
	}
	for _, tc := range cases {
		if got := normalizeDate(tc.in); got != tc.want {
			t.Errorf("normalizeDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRunUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/usage/summary" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{ // observer usage: period object + nested spend{amount,currency}
				"period": map[string]any{"from": "2026-05-28T00:00:00Z", "to": "2026-06-04T00:00:00Z"},
				"total": map[string]any{
					"spend":              map[string]any{"amount": "191.34", "currency": "CNY"},
					"requests":           1801,
					"tokens":             891000,
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
	code := Run([]string{"usage", "--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
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
