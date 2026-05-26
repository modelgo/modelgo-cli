package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

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

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"auth", "login",
		"--base-url", srv.URL,
		"--store", filepath.Join(t.TempDir(), "auth.json"),
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
}

func TestRunAuthLoginPrintsURLAndStoresCredential(t *testing.T) {
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

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"auth", "login",
		"--base-url", srv.URL,
		"--store", filepath.Join(t.TempDir(), "auth.json"),
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

func TestUsageMentionsAuth(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "auth login") {
		t.Fatalf("help missing auth login: %s", stdout.String())
	}
}
