package permissionscmd

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

func TestRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/account/permissions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{ // flat menus with name/visible/actions; plus context fields
				"active_tenant_id":   "ten_test123",
				"active_tenant_type": "personal",
				"region":             "domestic",
				"tenant_role":        "tenant_owner",
				"workspace_role":     "ws_owner",
				"granted":            []string{"dashboard:view", "models:view", "billing:view"},
				"menus": []map[string]any{
					{"key": "dashboard", "name": "Dashboard", "scope": "tenant", "visible": true, "actions": map[string]bool{"view": true}},
					{"key": "analytics", "name": "Analytics", "scope": "tenant", "visible": true, "actions": map[string]bool{"view": true}},
					{"key": "hidden", "name": "Hidden", "scope": "tenant", "visible": false, "actions": map[string]bool{}},
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
	if !bytes.Contains([]byte(out), []byte("dashboard:view")) {
		t.Errorf("expected granted permission in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("Analytics")) {
		t.Errorf("expected visible menu name in output, got: %s", out)
	}
	if bytes.Contains([]byte(out), []byte("Hidden")) {
		t.Errorf("non-visible menu must be filtered out, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("tenant_owner")) {
		t.Errorf("expected tenant role context in output, got: %s", out)
	}
}

func TestRun_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"granted": []string{"models:view"},
				"menus":   []any{},
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

	var result permissionsResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(result.Granted) != 1 || result.Granted[0] != "models:view" {
		t.Errorf("Granted = %v, want [models:view]", result.Granted)
	}
}
