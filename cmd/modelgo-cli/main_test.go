package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/config"
)

// writeTestEnvConfig pre-populates ~/.modelgo/config.json with a custom env
// named "test" pointing at the httptest server, and sets it active.
func writeTestEnvConfig(t *testing.T, dir, baseURL string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := config.Config{
		CurrentEnv: "test",
		Envs:       map[string]config.EnvEntry{"test": {BaseURL: baseURL}},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return cfgPath
}

func TestRunAuthLoginNoWaitJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/device/authorize" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"device_code":"device-1",
			"user_code":"ABCD-EFGH",
			"verification_url":"https://app.example/device?user_code=ABCD-EFGH",
			"expires_in":600,
			"interval":5
		}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfgPath := writeTestEnvConfig(t, dir, srv.URL)
	storePath := filepath.Join(dir, "auth.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"auth", "login",
		"--config", cfgPath,
		"--store", storePath,
		"--no-wait",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var body map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &body); err != nil {
		t.Fatalf("stdout not JSON: %v: %s", err, stdout.String())
	}
	if body["device_code"] != "device-1" || body["verification_url"] == "" {
		t.Fatalf("body=%v", body)
	}
	if body["event"] != "device_authorization" {
		t.Fatalf("event=%v", body["event"])
	}
	hint, _ := body["hint"].(string)
	if !strings.Contains(hint, "opaque string") || !strings.Contains(hint, "modelgo auth login --device-code device-1") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestRunAuthLoginPrintsURLAndStoresCredentialUnderEnv(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/device/authorize":
			_, _ = w.Write([]byte(`{
				"device_code":"device-1",
				"user_code":"ABCD-EFGH",
				"verification_url":"https://app.example/device?user_code=ABCD-EFGH",
				"expires_in":600,
				"interval":1
			}`))
		case "/v1/auth/device/token":
			_, _ = w.Write([]byte(`{
				"session_token":"sid_cli",
				"account_id":"acc_1",
				"tenant_id":"ten_1",
				"expires_in":3600,
				"token_type":"Session",
				"session_expires_at":"2026-05-26T10:00:00Z"
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfgPath := writeTestEnvConfig(t, dir, srv.URL)
	storePath := filepath.Join(dir, "auth.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"auth", "login",
		"--config", cfgPath,
		"--store", storePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "https://app.example/device") {
		t.Fatalf("stdout missing verification URL: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Logged in as acc_1") {
		t.Fatalf("stdout missing login success: %s", stdout.String())
	}
}

func TestRunAuthLoginBlockingJSONFirstEventCarriesAgentHint(t *testing.T) {
	t.Parallel()

	tokenCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/device/authorize":
			_, _ = w.Write([]byte(`{
				"device_code":"device-1",
				"user_code":"ABCD-EFGH",
				"verification_url":"https://app.example/device?user_code=ABCD-EFGH",
				"expires_in":600,
				"interval":1
			}`))
		case "/v1/auth/device/token":
			tokenCalls++
			_, _ = w.Write([]byte(`{
				"session_token":"sid_cli",
				"account_id":"acc_1",
				"tenant_id":"ten_1",
				"expires_in":3600,
				"token_type":"Session",
				"session_expires_at":"2026-05-26T10:00:00Z"
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfgPath := writeTestEnvConfig(t, dir, srv.URL)
	storePath := filepath.Join(dir, "auth.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"auth", "login",
		"--config", cfgPath,
		"--store", storePath,
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines=%d want 2 stdout=%s", len(lines), stdout.String())
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line not JSON: %v", err)
	}
	if first["event"] != "device_authorization" {
		t.Fatalf("first event=%v", first["event"])
	}
	hint, _ := first["agent_hint"].(string)
	if !strings.Contains(hint, "final turn messages") || !strings.Contains(hint, "modelgo auth login --device-code device-1") {
		t.Fatalf("agent_hint=%q", hint)
	}
}

func TestRunAuthStatusNotLoggedIn(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"auth", "status", "--store", filepath.Join(t.TempDir(), "missing.json")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Not logged in") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestUsageMentionsAuthAndEnv(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "auth login") {
		t.Fatalf("help missing auth login: %s", out)
	}
	if !strings.Contains(out, "env list") {
		t.Fatalf("help missing env subcommand: %s", out)
	}
	// Help text must say "modelgo" not "modelgo-cli".
	if strings.Contains(out, "modelgo-cli") {
		t.Fatalf("help still mentions modelgo-cli: %s", out)
	}
}

// A global --tenant override only applies to the tenant-scoped business
// commands; using it elsewhere is rejected (exit 2) rather than silently ignored.
func TestGlobalTenantRejectedForUnsupportedCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--tenant", "acme", "auth", "status"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (--tenant unsupported for auth); stderr: %s", code, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("--tenant is only supported")) {
		t.Errorf("expected guard message, got: %s", stderr.String())
	}
}

func TestAuthLoginHelpMentionsBlockingAndSplitFlow(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"auth", "login", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"device authorization login",
		"Block and poll until authorization completes",
		"modelgo auth login --no-wait --json",
		"modelgo auth login --device-code <DEVICE_CODE>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q: %s", want, out)
		}
	}
}

func TestRunEnvList(t *testing.T) {
	t.Parallel()
	cfgPath := writeTestEnvConfig(t, t.TempDir(), "https://api-test.modelgo.com")

	var stdout, stderr bytes.Buffer
	code := run([]string{"env", "list", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "cn") || !strings.Contains(out, "intl") || !strings.Contains(out, "test") {
		t.Fatalf("list output wrong: %s", out)
	}
}

func TestAuthLogoutSurfacesConfigParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte("{this is not valid json"), 0o600); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"auth", "logout", "--config", cfgPath, "--store", filepath.Join(dir, "auth.json")}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit on malformed config, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "load config") {
		t.Fatalf("stderr missing load-config error: %s", stderr.String())
	}
}

func TestAuthStatusSurfacesConfigParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte("{this is not valid json"), 0o600); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"auth", "status", "--config", cfgPath, "--store", filepath.Join(dir, "auth.json")}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit on malformed config, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "load config") {
		t.Fatalf("stderr missing load-config error: %s", stderr.String())
	}
}
