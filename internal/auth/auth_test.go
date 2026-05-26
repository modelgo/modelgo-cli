package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoginNoWaitReturnsDeviceInstructionsWithoutWritingCredential(t *testing.T) {
	t.Parallel()

	var sawScope string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/device/authorize" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body authorizeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		sawScope = body.Scope
		_ = json.NewEncoder(w).Encode(authorizeResponse{
			DeviceCode:      "device-1",
			UserCode:        "ABCD-EFGH",
			VerificationURL: "https://app.example/device?user_code=ABCD-EFGH",
			ExpiresIn:       600,
			Interval:        5,
		})
	}))
	defer srv.Close()

	storePath := filepath.Join(t.TempDir(), "auth.json")
	got, err := Login(context.Background(), Options{
		BaseURL:   srv.URL,
		Scope:     "api_keys:write usage:read",
		NoWait:    true,
		StorePath: storePath,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sawScope != "api_keys:write usage:read" {
		t.Fatalf("scope = %q", sawScope)
	}
	if got.DeviceCode != "device-1" || got.UserCode != "ABCD-EFGH" || got.VerificationURL == "" {
		t.Fatalf("result = %+v", got)
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("credential file should not exist after --no-wait, stat err=%v", err)
	}
}

func TestLoginWithDeviceCodePollsUntilApprovedAndStoresCredential(t *testing.T) {
	t.Parallel()

	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/device/token" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body tokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode token request: %v", err)
		}
		if body.DeviceCode != "device-1" {
			t.Fatalf("device_code=%q", body.DeviceCode)
		}
		polls++
		if polls == 1 {
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"symbol": "DEVICE_AUTHORIZATION_PENDING",
				"details": map[string]any{
					"interval": 1,
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{
			SessionToken:     "sid_cli",
			AccountID:        "acc_1",
			TenantID:         "ten_1",
			ExpiresIn:        3600,
			TokenType:        "Session",
			SessionExpiresAt: "2026-05-26T10:00:00Z",
		})
	}))
	defer srv.Close()

	storePath := filepath.Join(t.TempDir(), "nested", "auth.json")
	got, err := Login(context.Background(), Options{
		BaseURL:    srv.URL,
		DeviceCode: "device-1",
		StorePath:  storePath,
		PollDelay:  func(time.Duration) {},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !got.Authenticated || got.AccountID != "acc_1" || got.TenantID != "ten_1" {
		t.Fatalf("result = %+v", got)
	}
	if polls != 2 {
		t.Fatalf("polls=%d want 2", polls)
	}
	cred, err := LoadCredential(storePath)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if cred.SessionToken != "sid_cli" || cred.BaseURL != srv.URL || cred.AccountID != "acc_1" {
		t.Fatalf("credential = %+v", cred)
	}
	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("stat credential: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credential mode = %04o want 0600", perm)
	}
}

func TestCredentialStatusAndLogout(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "auth.json")
	if ok, _, err := Status(path); err != nil || ok {
		t.Fatalf("empty Status = (%v, _, %v), want false nil", ok, err)
	}
	if err := SaveCredential(path, Credential{
		BaseURL:      "https://permissions.example",
		SessionToken: "sid",
		AccountID:    "acc",
		TenantID:     "ten",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	ok, cred, err := Status(path)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !ok || cred.AccountID != "acc" {
		t.Fatalf("Status = (%v, %+v)", ok, cred)
	}
	if err := Logout(path); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if ok, _, err := Status(path); err != nil || ok {
		t.Fatalf("post-logout Status = (%v, _, %v), want false nil", ok, err)
	}
}

func TestDefaultCredentialPathUsesModelGoHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MODELGO_CLI_CONFIG_DIR", "")

	got := DefaultCredentialPath()
	want := filepath.Join(home, ".modelgo", "auth.json")
	if got != want {
		t.Fatalf("DefaultCredentialPath() = %q, want %q", got, want)
	}
}
