package balancecmd

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

	// Override the "cn" env to point at the test server.
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

func TestRunOverview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/tenants/ten_test123/balance" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"tenant_id":          "ten_test123",
				"balance":            1234.56,
				"frozen_balance":     200.0,
				"currency":           "CNY",
				"status":             "active",
				"auto_topup_enabled": true,
				"auto_topup_amount":  500.0,
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
	if !bytes.Contains([]byte(out), []byte("¥")) {
		t.Errorf("expected currency symbol ¥ in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("1234.56")) {
		t.Errorf("expected balance amount in output, got: %s", out)
	}
}

func TestRunOverview_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"tenant_id": "ten_test123",
				"balance":   100.0,
				"currency":  "USD",
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--json", "--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}

	var result balanceResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if result.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", result.Currency)
	}
}

// A global --tenant override (slug or id) selects that tenant's credential
// instead of the active one, resolving the slug to its tenant id.
func TestRunOverview_TenantOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "acme" must resolve to ten_test123 (the stored credential's id).
		if r.URL.Path != "/open/v1/tenants/ten_test123/balance" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{"tenant_id": "ten_test123", "balance": 1.0, "currency": "CNY"},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", cfgPath, "--store", storePath}, "acme", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
}

// An unknown --tenant must fail loudly (exit 1) rather than silently falling
// back to the active tenant.
func TestRunOverview_UnknownTenant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server must not be called for an unresolvable tenant: %s", r.URL.Path)
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", cfgPath, "--store", storePath}, "ghost", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (unknown tenant); stderr: %s", code, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("ghost")) {
		t.Errorf("expected error to mention the unknown tenant, got: %s", stderr.String())
	}
}

func TestRunTransactions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "consumption" {
			t.Errorf("type filter = %q, want consumption", r.URL.Query().Get("type"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": []map[string]any{
				{"id": "tx1", "type": "consumption", "amount": -0.15, "currency": "CNY", "description": "gpt-4o call"},
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"transactions", "--type", "consumption", "--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("tx1")) {
		t.Errorf("expected tx1 in output, got: %s", out)
	}
}

func TestRunGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"initial_grant":     100.0,
				"percent_remaining": 65.0,
				"depleted":          false,
			},
		})
	}))
	defer srv.Close()

	cfgPath, storePath := setupTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"grant", "--config", cfgPath, "--store", storePath}, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("65%")) {
		t.Errorf("expected 65%% in output, got: %s", out)
	}
}
