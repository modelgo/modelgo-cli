package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The real modelgo gateway wraps every response in a {code,msg,data} envelope
// (observed on the test backend). These tests pin the CLI's tolerance for that
// envelope while the older flat-shape tests above keep backward compatibility.

func TestLoginUnwrapsEnvelopedAuthorizeResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": authorizeResponse{
				DeviceCode:      "dev-env",
				UserCode:        "WXYZ-1234",
				VerificationURL: "https://app.example/device?user_code=WXYZ-1234",
				ExpiresIn:       600,
				Interval:        5,
			},
		})
	}))
	defer srv.Close()

	got, err := Login(context.Background(), Options{
		Env:       "test",
		BaseURL:   srv.URL,
		NoWait:    true,
		StorePath: filepath.Join(t.TempDir(), "auth.json"),
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got.DeviceCode != "dev-env" || got.UserCode != "WXYZ-1234" || got.VerificationURL == "" {
		t.Fatalf("result = %+v", got)
	}
}

func TestLoginTreatsEmptyEnvelopedTokenAsPending(t *testing.T) {
	t.Parallel()
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls++
		if polls == 1 {
			// Enveloped pending: HTTP 200, code 0, but data.session_token empty.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok", "data": tokenResponse{},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": tokenResponse{
				SessionToken:     "sid_env",
				AccountID:        "acc_e",
				TenantID:         "ten_e",
				ExpiresIn:        3600,
				TokenType:        "Session",
				SessionExpiresAt: "2026-06-01T10:00:00Z",
			},
		})
	}))
	defer srv.Close()

	got, err := Login(context.Background(), Options{
		Env:        "test",
		BaseURL:    srv.URL,
		DeviceCode: "dev-1",
		StorePath:  filepath.Join(t.TempDir(), "auth.json"),
		PollDelay:  func(time.Duration) {},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !got.Authenticated || got.TenantID != "ten_e" {
		t.Fatalf("result = %+v", got)
	}
	if polls < 2 {
		t.Fatalf("expected at least 2 polls (pending then success), got %d", polls)
	}
}

func TestLoginReturnsEnvelopedBusinessError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 40001, "msg": "invalid client", "data": nil,
		})
	}))
	defer srv.Close()

	_, err := Login(context.Background(), Options{
		Env:       "test",
		BaseURL:   srv.URL,
		NoWait:    true,
		StorePath: filepath.Join(t.TempDir(), "auth.json"),
	})
	if err == nil {
		t.Fatal("want error for business code != 0")
	}
	if !strings.Contains(err.Error(), "invalid client") {
		t.Fatalf("error = %v, want it to contain server msg", err)
	}
}

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

// TestLogoutActiveTenantFallbackIsDeterministic verifies the deterministic
// fallback rule when the active tenant is logged out:
//
//  1. prefer the prior PreviousTenantID if it still exists, else
//  2. the lexicographically smallest remaining tenant id (sort.Strings min),
//     else empty when nothing remains.
//
// Login order ten_1, ten_2, ten_3 leaves active=ten_3, previous=ten_2.
func TestLogoutActiveTenantFallbackIsDeterministic(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_2"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s3", TenantID: "ten_3"})

	// active=ten_3, previous=ten_2 -> logging out ten_3 falls back to previous.
	active, _ := LoadActive("cn", path)
	if active.TenantID != "ten_3" {
		t.Fatalf("precondition active = %q, want ten_3", active.TenantID)
	}
	if err := Logout("cn", "ten_3", path); err != nil {
		t.Fatalf("Logout ten_3: %v", err)
	}
	active, err := LoadActive("cn", path)
	if err != nil || active.TenantID != "ten_2" {
		t.Fatalf("after logout ten_3, active = %+v err=%v, want ten_2 (previous)", active, err)
	}

	// Now active=ten_2, previous was cleared (it equaled the new active).
	// Logging out ten_2 has no usable previous, so it falls back to the
	// lexicographically smallest remaining id: ten_1.
	if err := Logout("cn", "ten_2", path); err != nil {
		t.Fatalf("Logout ten_2: %v", err)
	}
	active, err = LoadActive("cn", path)
	if err != nil || active.TenantID != "ten_1" {
		t.Fatalf("after logout ten_2, active = %+v err=%v, want ten_1 (sorted min)", active, err)
	}

	// Logging out the last tenant clears the env entirely.
	if err := Logout("cn", "ten_1", path); err != nil {
		t.Fatalf("Logout ten_1: %v", err)
	}
	if _, err := LoadActive("cn", path); !os.IsNotExist(err) {
		t.Fatalf("env should be gone after last logout, got err=%v", err)
	}
}

// TestLogoutSortedFallbackWhenPreviousMissing covers the branch where the
// PreviousTenantID points at a tenant that is not the active one but no longer
// exists: fallback must use the sorted-smallest remaining id, not previous.
func TestLogoutSortedFallbackWhenPreviousMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_b"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_a"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s3", TenantID: "ten_c"})
	// active=ten_c, previous=ten_a.

	// Remove the previous tenant first; this leaves a dangling previous pointer.
	if err := Logout("cn", "ten_a", path); err != nil {
		t.Fatalf("Logout ten_a: %v", err)
	}
	// active is still ten_c; previous now points at a removed tenant.
	active, _ := LoadActive("cn", path)
	if active.TenantID != "ten_c" {
		t.Fatalf("active = %q, want ten_c", active.TenantID)
	}

	// Logging out active ten_c: previous (ten_a) is gone, so fall back to the
	// sorted-smallest remaining id, which is ten_b.
	if err := Logout("cn", "ten_c", path); err != nil {
		t.Fatalf("Logout ten_c: %v", err)
	}
	active, err := LoadActive("cn", path)
	if err != nil || active.TenantID != "ten_b" {
		t.Fatalf("after logout ten_c, active = %+v err=%v, want ten_b (sorted min)", active, err)
	}
}

// TestListTenantsIsSortedByTenantID asserts ListTenants returns a deterministic
// order by tenant id regardless of login order.
func TestListTenantsIsSortedByTenantID(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "auth.json")
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_3"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_1"})
	_ = SaveCredential(path, Credential{Env: "cn", SessionToken: "s3", TenantID: "ten_2"})

	creds, _, err := ListTenants("cn", path)
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	want := []string{"ten_1", "ten_2", "ten_3"}
	if len(creds) != len(want) {
		t.Fatalf("len = %d, want %d", len(creds), len(want))
	}
	for i, w := range want {
		if creds[i].TenantID != w {
			t.Fatalf("creds[%d] = %q, want %q (sorted)", i, creds[i].TenantID, w)
		}
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

// FetchTenants must surface a {code,msg} business-error envelope (which the
// gateway returns with HTTP 200) as a clean error, not a low-level JSON
// unmarshal failure — observed live as `tenant list --remote` against test.
func TestFetchTenantsEnvelopeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":40401,"msg":"resource not found","data":null,"request_id":"x"}`))
	}))
	defer srv.Close()

	_, err := FetchTenants(context.Background(), srv.Client(), &Credential{BaseURL: srv.URL, SessionToken: "t"})
	if err == nil {
		t.Fatal("expected error for {code:40401} envelope, got nil")
	}
	if !strings.Contains(err.Error(), "resource not found") || !strings.Contains(err.Error(), "40401") {
		t.Fatalf("error = %q, want it to name the msg and code", err.Error())
	}
	if strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("error leaked a low-level decode failure: %q", err.Error())
	}
}

// FetchTenants accepts both the {code:0,data:[...]} success envelope and a bare
// [...] array (backward compatibility).
func TestFetchTenantsAcceptsEnvelopeAndBareArray(t *testing.T) {
	for _, tc := range []struct {
		name, body string
	}{
		{"envelope", `{"code":0,"msg":"ok","data":[{"tenant_id":"ten_1","slug":"acme"}]}`},
		{"bare", `[{"tenant_id":"ten_1","slug":"acme"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			got, err := FetchTenants(context.Background(), srv.Client(), &Credential{BaseURL: srv.URL, SessionToken: "t"})
			if err != nil {
				t.Fatalf("FetchTenants: %v", err)
			}
			if len(got) != 1 || got[0].TenantID != "ten_1" || got[0].Slug != "acme" {
				t.Fatalf("got %+v, want one tenant ten_1/acme", got)
			}
		})
	}
}
