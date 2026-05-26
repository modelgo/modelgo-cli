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
		Env:       "test",
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

func TestLoginStoresCredentialUnderEnvBucket(t *testing.T) {
	t.Parallel()

	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/device/token" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		polls++
		if polls == 1 {
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"symbol": "DEVICE_AUTHORIZATION_PENDING"})
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
		Env:        "test",
		BaseURL:    srv.URL,
		DeviceCode: "device-1",
		StorePath:  storePath,
		PollDelay:  func(time.Duration) {},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !got.Authenticated || got.AccountID != "acc_1" {
		t.Fatalf("result = %+v", got)
	}

	cred, err := LoadCredential("test", storePath)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if cred.Env != "test" || cred.SessionToken != "sid_cli" || cred.BaseURL != srv.URL {
		t.Fatalf("credential = %+v", cred)
	}

	// Other envs should report no credential.
	if _, err := LoadCredential("cn", storePath); !os.IsNotExist(err) {
		t.Fatalf("LoadCredential(cn) = %v, want not exist", err)
	}
}

func TestSaveCredentialPreservesOtherEnvs(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "auth.json")
	if err := SaveCredential(path, Credential{Env: "cn", SessionToken: "sid-cn", AccountID: "a"}); err != nil {
		t.Fatalf("save cn: %v", err)
	}
	if err := SaveCredential(path, Credential{Env: "test", SessionToken: "sid-test", AccountID: "b"}); err != nil {
		t.Fatalf("save test: %v", err)
	}

	cn, err := LoadCredential("cn", path)
	if err != nil || cn.SessionToken != "sid-cn" {
		t.Fatalf("cn = %+v err=%v", cn, err)
	}
	test, err := LoadCredential("test", path)
	if err != nil || test.SessionToken != "sid-test" {
		t.Fatalf("test = %+v err=%v", test, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %04o", perm)
	}
}

func TestLogoutRemovesOnlyTheNamedBucket(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "sid-cn"})
	_ = SaveCredential(path, Credential{Env: "test", SessionToken: "sid-test"})

	if err := Logout("cn", path); err != nil {
		t.Fatalf("Logout cn: %v", err)
	}
	if _, err := LoadCredential("cn", path); !os.IsNotExist(err) {
		t.Fatalf("cn still present: %v", err)
	}
	test, err := LoadCredential("test", path)
	if err != nil || test.SessionToken != "sid-test" {
		t.Fatalf("test bucket damaged: %+v err=%v", test, err)
	}
}

func TestLogoutAllRemovesFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "sid-cn"})
	if err := LogoutAll(path); err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestStatusReturnsCredentialForEnv(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")

	if ok, _, err := Status("cn", path); err != nil || ok {
		t.Fatalf("empty Status = (%v, _, %v)", ok, err)
	}

	_ = SaveCredential(path, Credential{
		Env:          "cn",
		BaseURL:      "https://api.modelgo.com",
		SessionToken: "sid",
		AccountID:    "acc",
		ExpiresAt:    time.Now().Add(time.Hour),
	})

	ok, cred, err := Status("cn", path)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !ok || cred.AccountID != "acc" {
		t.Fatalf("Status = (%v, %+v)", ok, cred)
	}

	if ok, _, _ := Status("test", path); ok {
		t.Fatal("test env should not be logged in")
	}
}

func TestLoadCredentialMigratesLegacyFlatFormat(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")

	// Write the old flat format (pre-bucket).
	legacy := []byte(`{
  "base_url": "https://api.modelgo.com",
  "session_token": "legacy-sid",
  "account_id": "acc",
  "tenant_id": "ten",
  "token_type": "Session"
}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cred, err := LoadCredential("cn", path)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if cred.SessionToken != "legacy-sid" || cred.AccountID != "acc" {
		t.Fatalf("migrated cred = %+v", cred)
	}
	if cred.Env != "cn" {
		t.Fatalf("expected env=cn after migration, got %q", cred.Env)
	}

	// Loading a non-cn env from a migrated file should report not found.
	if _, err := LoadCredential("test", path); !os.IsNotExist(err) {
		t.Fatalf("non-cn after migration = %v, want not exist", err)
	}
}

func TestDefaultCredentialPathUsesModelGoHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got := DefaultCredentialPath()
	want := filepath.Join(home, ".modelgo", "auth.json")
	if got != want {
		t.Fatalf("DefaultCredentialPath() = %q, want %q", got, want)
	}
}

func TestLoadCredentialBackfillsEnvFromBucketKey(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	// Hand-written file missing inner "env" field for a bucket.
	raw := []byte(`{
  "cn": {
    "session_token": "sid-cn",
    "account_id": "acc"
  }
}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cred, err := LoadCredential("cn", path)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if cred.Env != "cn" {
		t.Fatalf("Env = %q, want cn (backfilled from bucket key)", cred.Env)
	}
}

func TestLoadCredentialAllowsEnvNamedSessionToken(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	// Bucketed file where the env name happens to be "session_token".
	// The flat-format detector must NOT misclassify this.
	if err := SaveCredential(path, Credential{
		Env:          "session_token",
		SessionToken: "sid-1",
		AccountID:    "acc",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	cred, err := LoadCredential("session_token", path)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if cred.SessionToken != "sid-1" || cred.Env != "session_token" {
		t.Fatalf("cred = %+v", cred)
	}
}

func TestLoadStoreOnEmptyObjectReturnsEmptyStore(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadCredential("cn", path); !os.IsNotExist(err) {
		t.Fatalf("LoadCredential(cn) on {} = %v, want IsNotExist", err)
	}
}
