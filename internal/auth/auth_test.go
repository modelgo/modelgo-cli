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

	var sawScope, sawClientName, sawUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/device/authorize" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawUserAgent = r.Header.Get("User-Agent")
		var body authorizeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		sawScope = body.Scope
		sawClientName = body.ClientName
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
	if sawClientName != "modelgo" {
		t.Fatalf("client_name = %q, want modelgo", sawClientName)
	}
	if sawUserAgent != "modelgo" {
		t.Fatalf("User-Agent = %q, want modelgo", sawUserAgent)
	}
	if got.DeviceCode != "device-1" || got.UserCode != "ABCD-EFGH" || got.VerificationURL == "" {
		t.Fatalf("result = %+v", got)
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("credential file should not exist after --no-wait, stat err=%v", err)
	}
}

func TestLoginStoresCredentialUnderEnvTenantBucket(t *testing.T) {
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
	if !got.Authenticated || got.AccountID != "acc_1" || got.TenantID != "ten_1" {
		t.Fatalf("result = %+v", got)
	}

	cred, err := LoadCredential("test", "ten_1", storePath)
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if cred.Env != "test" || cred.SessionToken != "sid_cli" || cred.BaseURL != srv.URL {
		t.Fatalf("credential = %+v", cred)
	}

	// Active pointer resolves to the freshly logged-in tenant.
	active, err := LoadActive("test", storePath)
	if err != nil {
		t.Fatalf("LoadActive: %v", err)
	}
	if active.TenantID != "ten_1" {
		t.Fatalf("active = %q, want ten_1", active.TenantID)
	}

	// Other envs should report no credential.
	if _, err := LoadActive("cn", storePath); !os.IsNotExist(err) {
		t.Fatalf("LoadActive(cn) = %v, want not exist", err)
	}
}

func TestSaveCredentialBucketsByTenantAndSetsActive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	must(SaveCredential(path, Credential{Env: "cn", BaseURL: "https://api", SessionToken: "s1", AccountID: "a1", TenantID: "ten_1", TenantSlug: "acme", TokenType: "Session"}))
	must(SaveCredential(path, Credential{Env: "cn", BaseURL: "https://api", SessionToken: "s2", AccountID: "a1", TenantID: "ten_2", TenantSlug: "globex", TokenType: "Session"}))

	// 两份并存
	c1, err := LoadCredential("cn", "ten_1", path)
	must(err)
	if c1.SessionToken != "s1" {
		t.Fatalf("ten_1 token = %q", c1.SessionToken)
	}
	c2, err := LoadCredential("cn", "ten_2", path)
	must(err)
	if c2.SessionToken != "s2" {
		t.Fatalf("ten_2 token = %q", c2.SessionToken)
	}

	// 最后保存的成为活跃
	active, err := LoadActive("cn", path)
	must(err)
	if active.TenantID != "ten_2" {
		t.Fatalf("active = %q, want ten_2", active.TenantID)
	}

	// 切回 ten_1，记录 previous
	must(UseTenant("cn", "ten_1", path))
	active, _ = LoadActive("cn", path)
	if active.TenantID != "ten_1" {
		t.Fatalf("active after use = %q", active.TenantID)
	}

	// use - 回到上一个
	must(UsePreviousTenant("cn", path))
	active, _ = LoadActive("cn", path)
	if active.TenantID != "ten_2" {
		t.Fatalf("active after use- = %q", active.TenantID)
	}
}

func TestUseTenantUnknownReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1"})
	if err := UseTenant("cn", "ten_nope", path); err == nil {
		t.Fatal("want error switching to a tenant not logged in")
	}
}

func TestListTenantsReturnsAllAndActive(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1", TenantSlug: "acme"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_2", TenantSlug: "globex"})

	creds, active, err := ListTenants("cn", path)
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("len = %d, want 2", len(creds))
	}
	if active != "ten_2" {
		t.Fatalf("active = %q, want ten_2", active)
	}

	// Missing file returns empty without error.
	creds, active, err = ListTenants("cn", filepath.Join(t.TempDir(), "missing.json"))
	if err != nil || len(creds) != 0 || active != "" {
		t.Fatalf("missing file: creds=%v active=%q err=%v", creds, active, err)
	}
}

func TestResolveTenantIDBySlugOrID(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1", TenantSlug: "acme"})

	id, err := ResolveTenantID("cn", "acme", path)
	if err != nil || id != "ten_1" {
		t.Fatalf("by slug = %q err=%v", id, err)
	}
	id, err = ResolveTenantID("cn", "ten_1", path)
	if err != nil || id != "ten_1" {
		t.Fatalf("by id = %q err=%v", id, err)
	}
	if _, err := ResolveTenantID("cn", "nope", path); err == nil {
		t.Fatal("want error for unknown tenant")
	}
}

func TestResolveActiveOrFlag(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1", TenantSlug: "acme"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_2", TenantSlug: "globex"})

	// No flag -> active (ten_2, last saved).
	cred, err := ResolveActiveOrFlag("cn", "", path)
	if err != nil || cred.TenantID != "ten_2" {
		t.Fatalf("active = %+v err=%v", cred, err)
	}
	// Flag overrides to ten_1 without changing active.
	cred, err = ResolveActiveOrFlag("cn", "acme", path)
	if err != nil || cred.TenantID != "ten_1" {
		t.Fatalf("flag = %+v err=%v", cred, err)
	}
	active, _ := LoadActive("cn", path)
	if active.TenantID != "ten_2" {
		t.Fatalf("flag override must not change active, got %q", active.TenantID)
	}
}

func TestSaveCredentialPreservesOtherEnvs(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "auth.json")
	if err := SaveCredential(path, Credential{Env: "cn", SessionToken: "sid-cn", AccountID: "a", TenantID: "ten_cn"}); err != nil {
		t.Fatalf("save cn: %v", err)
	}
	if err := SaveCredential(path, Credential{Env: "test", SessionToken: "sid-test", AccountID: "b", TenantID: "ten_test"}); err != nil {
		t.Fatalf("save test: %v", err)
	}

	cn, err := LoadActive("cn", path)
	if err != nil || cn.SessionToken != "sid-cn" {
		t.Fatalf("cn = %+v err=%v", cn, err)
	}
	test, err := LoadActive("test", path)
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

func TestLogoutRemovesOnlyTheNamedTenant(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_2"})

	// Remove only ten_2 (which is active); ten_1 survives and becomes active.
	if err := Logout("cn", "ten_2", path); err != nil {
		t.Fatalf("Logout ten_2: %v", err)
	}
	if _, err := LoadCredential("cn", "ten_2", path); !os.IsNotExist(err) {
		t.Fatalf("ten_2 still present: %v", err)
	}
	active, err := LoadActive("cn", path)
	if err != nil || active.TenantID != "ten_1" {
		t.Fatalf("active after logout = %+v err=%v", active, err)
	}
}

func TestLogoutEntireEnv(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1"})
	_ = SaveCredential(path, Credential{Env: "test", SessionToken: "s2", TenantID: "ten_2"})

	if err := Logout("cn", "", path); err != nil {
		t.Fatalf("Logout cn: %v", err)
	}
	if _, err := LoadActive("cn", path); !os.IsNotExist(err) {
		t.Fatalf("cn still present: %v", err)
	}
	test, err := LoadActive("test", path)
	if err != nil || test.SessionToken != "s2" {
		t.Fatalf("test bucket damaged: %+v err=%v", test, err)
	}
}

func TestLogoutAllRemovesFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "sid-cn", TenantID: "ten_1"})
	if err := LogoutAll(path); err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestStatusReturnsActiveCredentialForEnv(t *testing.T) {
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
		TenantID:     "ten_1",
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

func TestDefaultCredentialPathUsesModelGoHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got := DefaultCredentialPath()
	want := filepath.Join(home, ".modelgo", "auth.json")
	if got != want {
		t.Fatalf("DefaultCredentialPath() = %q, want %q", got, want)
	}
}

func TestLoadActiveOnEmptyObjectReturnsNotExist(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadActive("cn", path); !os.IsNotExist(err) {
		t.Fatalf("LoadActive(cn) on {} = %v, want IsNotExist", err)
	}
}
