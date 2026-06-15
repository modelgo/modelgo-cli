package tenantcmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/auth"
)

func TestTenantListMarksActive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	_ = auth.SaveCredential(path, auth.Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1", TenantSlug: "acme"})
	_ = auth.SaveCredential(path, auth.Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_2", TenantSlug: "globex"})

	var buf bytes.Buffer
	if err := runList(&buf, "cn", path); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "globex") || !strings.Contains(out, "* ten_2") {
		t.Fatalf("active tenant not marked: %s", out)
	}
	if !strings.Contains(out, "  ten_1") {
		t.Fatalf("inactive tenant not listed with plain marker: %s", out)
	}
}

func TestTenantListEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := runList(&buf, "cn", filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No tenants") {
		t.Fatalf("expected empty message, got: %s", buf.String())
	}
}

func TestRunUseSwitchesActive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	_ = auth.SaveCredential(path, auth.Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1", TenantSlug: "acme"})
	_ = auth.SaveCredential(path, auth.Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_2", TenantSlug: "globex"})

	var stdout, stderr bytes.Buffer
	// use by slug
	if code := Run([]string{"use", "acme", "--env", "cn", "--store", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("use acme exit=%d stderr=%s", code, stderr.String())
	}
	active, _ := auth.LoadActive("cn", path)
	if active.TenantID != "ten_1" {
		t.Fatalf("active = %q, want ten_1", active.TenantID)
	}

	// use - returns to previous (ten_2)
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"use", "-", "--env", "cn", "--store", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("use - exit=%d stderr=%s", code, stderr.String())
	}
	active, _ = auth.LoadActive("cn", path)
	if active.TenantID != "ten_2" {
		t.Fatalf("active after use- = %q, want ten_2", active.TenantID)
	}
}

func TestRunUseUnknownTenantFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	_ = auth.SaveCredential(path, auth.Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1", TenantSlug: "acme"})

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"use", "nope", "--env", "cn", "--store", path}, &stdout, &stderr); code == 0 {
		t.Fatalf("expected non-zero exit for unknown tenant, stdout=%s", stdout.String())
	}
}

func TestTenantListJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	_ = auth.SaveCredential(path, auth.Credential{Env: "cn", SessionToken: "s1", TenantID: "ten_1", TenantSlug: "acme", TenantName: "Acme Corp"})
	_ = auth.SaveCredential(path, auth.Credential{Env: "cn", SessionToken: "s2", TenantID: "ten_2", TenantSlug: "globex"})

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"list", "--json", "--env", "cn", "--store", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"tenant_id"`) || !strings.Contains(out, `"acme"`) {
		t.Fatalf("unexpected JSON output: %s", out)
	}
	// active tenant (ten_2) should have active:true
	if !strings.Contains(out, `"active":true`) {
		t.Fatalf("active flag missing in JSON: %s", out)
	}
}
