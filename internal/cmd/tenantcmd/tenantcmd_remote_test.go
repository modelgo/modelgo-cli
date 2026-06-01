package tenantcmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/auth"
)

func TestTenantListRemoteMergesLocalAndRemote(t *testing.T) {
	t.Parallel()

	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/v1/tenants" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"tenant_id": "ten_1", "name": "Acme Inc", "slug": "acme", "role": "owner", "is_default": true},
				{"tenant_id": "ten_3", "name": "Initech", "slug": "initech", "role": "member", "is_default": false},
			},
		})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "auth.json")
	// Locally logged into ten_1 only; ten_3 exists remotely but not locally.
	_ = auth.SaveCredential(path, auth.Credential{
		Env: "cn", BaseURL: srv.URL, SessionToken: "sid_active",
		TenantID: "ten_1", TenantSlug: "acme",
	})

	var buf, errBuf bytes.Buffer
	if err := runListRemote(&buf, &errBuf, "cn", path); err != nil {
		t.Fatalf("runListRemote: %v", err)
	}
	out := buf.String()

	if sawAuth != "Bearer sid_active" {
		t.Fatalf("Authorization = %q, want Bearer sid_active", sawAuth)
	}
	// Remote role surfaced.
	if !strings.Contains(out, "owner") || !strings.Contains(out, "member") {
		t.Fatalf("roles missing: %s", out)
	}
	// Login status markers.
	if !strings.Contains(out, "ten_1") || !strings.Contains(out, "ten_3") {
		t.Fatalf("tenants missing: %s", out)
	}
	if !strings.Contains(out, "initech") || !strings.Contains(out, "login") {
		t.Fatalf("not-logged-in tenant should be flagged to login: %s", out)
	}
}

func TestTenantListRemoteWithoutActiveSession(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.json")
	var buf, errBuf bytes.Buffer
	if err := runListRemote(&buf, &errBuf, "cn", path); err == nil {
		t.Fatal("want error when no active session to authenticate with")
	}
}
