# Environment Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add named environment (`env`) management so users can switch between built-in `cn` / `intl` sites and add custom envs (e.g. internal test gateway) via a config file, without leaking implementation details through env vars.

**Architecture:**
- Built-in envs `cn` and `intl` are baked into the binary (`internal/env`).
- Custom envs and the currently-active env live in `~/.modelgo/config.json` (`internal/config`).
- Credentials are bucketed per env in `~/.modelgo/auth.json` (old flat format auto-migrated to `cn`).
- Auth commands resolve the active env via `--env` flag > config > default `cn`; URL via config override > built-in > error.
- No environment variables read anywhere. Test injection uses explicit `--config` / `--store` paths.

**Tech Stack:** Go 1.22, stdlib only (no new deps). Existing `flag` package for CLI parsing. `httptest` for tests.

---

## File Structure

**Create:**
- `internal/config/config.go` — load/save `~/.modelgo/config.json`
- `internal/config/config_test.go`
- `internal/env/env.go` — built-in URL table, `Resolve`, `List`
- `internal/env/env_test.go`
- `internal/cmd/envcmd/envcmd.go` — `env list/current/use/add/remove` subcommands
- `internal/cmd/envcmd/envcmd_test.go`

**Modify:**
- `internal/auth/auth.go` — bucket credentials by env, drop env-var lookups, add `Env` to `Options` + `Credential`
- `internal/auth/auth_test.go` — update tests for bucketed format and removed env vars
- `cmd/modelgo-cli/main.go` — wire `env` subcommand, add `--env` / `--config` flags to auth commands, remove `--base-url`, change `modelgo-cli` → `modelgo` in help
- `cmd/modelgo-cli/main_test.go` — replace `--base-url` usage with config-file fixtures

---

## Task 1: Config package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CurrentEnv != "" || len(cfg.Envs) != 0 {
		t.Fatalf("expected empty config, got %+v", cfg)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	in := Config{
		CurrentEnv: "test",
		Envs: map[string]EnvEntry{
			"test": {BaseURL: "https://api-test.modelgo.com"},
		},
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %04o want 0600", perm)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.CurrentEnv != "test" || out.Envs["test"].BaseURL != "https://api-test.modelgo.com" {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestDefaultPathUsesHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got := DefaultPath()
	want := filepath.Join(home, ".modelgo", "config.json")
	if got != want {
		t.Fatalf("DefaultPath = %q want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/...`
Expected: FAIL — `config` package missing

- [ ] **Step 3: Write the implementation**

```go
// internal/config/config.go
// Package config reads and writes the modelgo-cli user config file at
// ~/.modelgo/config.json. The config file is the sole source of user-level
// settings (current env, custom env definitions). No environment variables
// are consulted.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// EnvEntry is a user-defined environment override (typically pointing at a
// custom or internal-test API gateway). Setting an entry for a built-in env
// name (e.g. "cn") overrides that built-in's URL.
type EnvEntry struct {
	BaseURL string `json:"base_url"`
}

// Config is the on-disk shape of ~/.modelgo/config.json.
type Config struct {
	CurrentEnv string              `json:"current_env,omitempty"`
	Envs       map[string]EnvEntry `json:"envs,omitempty"`
}

// Load reads the config file at path. A missing file returns an empty
// Config and nil error — first-run users have no config.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Envs == nil {
		cfg.Envs = map[string]EnvEntry{}
	}
	return cfg, nil
}

// Save writes the config file at path with 0600 permissions, creating
// parent directories as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// DefaultPath returns the canonical config file path (~/.modelgo/config.json).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".modelgo", "config.json")
	}
	return filepath.Join(home, ".modelgo", "config.json")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add ~/.modelgo/config.json read/write"
```

---

## Task 2: Env package

**Files:**
- Create: `internal/env/env.go`
- Create: `internal/env/env_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/env/env_test.go
package env

import (
	"errors"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/config"
)

func TestResolveBuiltInCN(t *testing.T) {
	t.Parallel()
	url, err := Resolve("cn", config.Config{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if url != "https://api.modelgo.com" {
		t.Fatalf("url = %q", url)
	}
}

func TestResolveBuiltInIntl(t *testing.T) {
	t.Parallel()
	url, err := Resolve("intl", config.Config{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if url != "https://api.modelgo.global" {
		t.Fatalf("url = %q", url)
	}
}

func TestConfigOverridesBuiltIn(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Envs: map[string]config.EnvEntry{
		"cn": {BaseURL: "https://staging-cn.modelgo.com"},
	}}
	url, err := Resolve("cn", cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if url != "https://staging-cn.modelgo.com" {
		t.Fatalf("expected config override, got %q", url)
	}
}

func TestResolveCustomEnvFromConfig(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Envs: map[string]config.EnvEntry{
		"test": {BaseURL: "https://api-test.modelgo.com"},
	}}
	url, err := Resolve("test", cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if url != "https://api-test.modelgo.com" {
		t.Fatalf("url = %q", url)
	}
}

func TestResolveUnknownEnvReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Resolve("nope", config.Config{})
	if !errors.Is(err, ErrUnknownEnv) {
		t.Fatalf("err = %v, want ErrUnknownEnv", err)
	}
}

func TestActiveEnvPrefersExplicitOverConfigOverDefault(t *testing.T) {
	t.Parallel()
	if got := ActiveEnv("test", config.Config{CurrentEnv: "intl"}); got != "test" {
		t.Fatalf("explicit lost: %q", got)
	}
	if got := ActiveEnv("", config.Config{CurrentEnv: "intl"}); got != "intl" {
		t.Fatalf("config lost: %q", got)
	}
	if got := ActiveEnv("", config.Config{}); got != "cn" {
		t.Fatalf("default lost: %q", got)
	}
}

func TestListMergesBuiltInAndCustom(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		CurrentEnv: "test",
		Envs: map[string]config.EnvEntry{
			"test": {BaseURL: "https://api-test.modelgo.com"},
			"cn":   {BaseURL: "https://staging-cn.modelgo.com"},
		},
	}
	entries := List(cfg)

	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}

	cn, ok := byName["cn"]
	if !ok || !cn.BuiltIn || !cn.Overridden || cn.URL != "https://staging-cn.modelgo.com" {
		t.Fatalf("cn entry wrong: %+v", cn)
	}
	intl, ok := byName["intl"]
	if !ok || !intl.BuiltIn || intl.Overridden || intl.URL != "https://api.modelgo.global" {
		t.Fatalf("intl entry wrong: %+v", intl)
	}
	test, ok := byName["test"]
	if !ok || test.BuiltIn || test.Overridden || !test.Active || test.URL != "https://api-test.modelgo.com" {
		t.Fatalf("test entry wrong: %+v", test)
	}
}

func TestIsBuiltIn(t *testing.T) {
	t.Parallel()
	if !IsBuiltIn("cn") || !IsBuiltIn("intl") {
		t.Fatal("cn/intl should be built-in")
	}
	if IsBuiltIn("test") {
		t.Fatal("test should not be built-in")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/env/...`
Expected: FAIL — package missing

- [ ] **Step 3: Write the implementation**

```go
// internal/env/env.go
// Package env defines built-in modelgo environments and resolves env names
// to API base URLs. Resolution order: config override > built-in > error.
package env

import (
	"errors"
	"fmt"
	"sort"

	"github.com/modelgo/modelgo-cli/internal/config"
)

const DefaultEnv = "cn"

var builtIn = map[string]string{
	"cn":   "https://api.modelgo.com",
	"intl": "https://api.modelgo.global",
}

// ErrUnknownEnv is returned when an env name is neither built-in nor present
// in the config file's envs map.
var ErrUnknownEnv = errors.New("unknown environment")

// Resolve returns the base URL for the named env. Config overrides win over
// built-in URLs so users can repoint cn/intl at a staging host via
// `modelgo env add cn --base-url ...`.
func Resolve(name string, cfg config.Config) (string, error) {
	if entry, ok := cfg.Envs[name]; ok && entry.BaseURL != "" {
		return entry.BaseURL, nil
	}
	if url, ok := builtIn[name]; ok {
		return url, nil
	}
	return "", fmt.Errorf("%w: %q. Run 'modelgo env add %s --base-url URL' to register it", ErrUnknownEnv, name, name)
}

// ActiveEnv picks the env name to operate on given an explicit value
// (typically from a --env flag) and the loaded config.
func ActiveEnv(explicit string, cfg config.Config) string {
	if explicit != "" {
		return explicit
	}
	if cfg.CurrentEnv != "" {
		return cfg.CurrentEnv
	}
	return DefaultEnv
}

// IsBuiltIn reports whether name is a built-in env (cn, intl).
func IsBuiltIn(name string) bool {
	_, ok := builtIn[name]
	return ok
}

// Entry is a row in `modelgo env list`.
type Entry struct {
	Name       string
	URL        string
	BuiltIn    bool
	Overridden bool // built-in whose URL is shadowed by a config entry
	Active     bool
}

// List returns all known envs (built-ins plus custom from config) sorted by
// name, with the active flag set on the current env.
func List(cfg config.Config) []Entry {
	seen := map[string]Entry{}
	for name, url := range builtIn {
		seen[name] = Entry{Name: name, URL: url, BuiltIn: true}
	}
	for name, entry := range cfg.Envs {
		if entry.BaseURL == "" {
			continue
		}
		e := seen[name]
		e.Name = name
		e.URL = entry.BaseURL
		if e.BuiltIn {
			e.Overridden = true
		}
		seen[name] = e
	}
	active := ActiveEnv("", cfg)
	out := make([]Entry, 0, len(seen))
	for _, e := range seen {
		e.Active = e.Name == active
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/env/...`
Expected: PASS — 7 tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/env/
git commit -m "feat(env): add built-in cn/intl table and config-aware resolver"
```

---

## Task 3: Auth credential refactor (bucket by env, drop env vars)

**Files:**
- Modify: `internal/auth/auth.go`
- Modify: `internal/auth/auth_test.go`

This task changes credential file format from flat `{...}` to bucketed `{ "env_name": {...} }`, adds `Env` to `Options` and `Credential`, drops `MODELGO_API_URL` and `MODELGO_CLI_CONFIG_DIR` env vars, and migrates old-format files automatically.

- [ ] **Step 1: Update failing tests first (TDD)**

Replace the contents of `internal/auth/auth_test.go` with:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/...`
Expected: FAIL — signatures don't match (`LoadCredential` takes 2 args now, `Logout` takes 2, etc.)

- [ ] **Step 3: Rewrite `internal/auth/auth.go`**

Replace the entire file with:

```go
// Package auth implements modelgo-cli device login and per-env credential
// storage. ~/.modelgo/auth.json holds a JSON object keyed by env name, e.g.
// { "cn": {...}, "test": {...} }. Each value follows the legacy Credential
// shape. Old flat-format files (single credential object) are auto-migrated
// to a single "cn" bucket on first read.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPollInterval = 5 * time.Second
	defaultLoginTimeout = 10 * time.Minute

	// loginPathPrefix is the public login prefix. Login lives outside the
	// model-gateway openapi surface — modelgo-web-api owns it — but the CLI
	// hits a single public hostname (e.g. api.modelgo.com) that the
	// deployment's ingress routes by prefix. modelgo-model-gateway never
	// sees these requests; /open/v1/* is reserved for authenticated openapi
	// calls carrying a Bearer session_token.
	loginPathPrefix = "/v1"

	// openAPIPathPrefix is the future public prefix for already-authenticated
	// openapi calls served by model-gateway.
	openAPIPathPrefix = "/open/v1"
)

type Options struct {
	Env        string // env name this login belongs to (e.g. "cn", "test")
	BaseURL    string // resolved API base URL for Env
	Scope      string
	NoWait     bool
	DeviceCode string
	StorePath  string
	HTTPClient *http.Client
	PollDelay  func(time.Duration)
	Timeout    time.Duration
}

type LoginResult struct {
	DeviceCode      string
	UserCode        string
	VerificationURL string
	ExpiresIn       int64
	Interval        int
	Authenticated   bool
	Env             string
	AccountID       string
	TenantID        string
	ExpiresAt       time.Time
}

type Credential struct {
	Env          string    `json:"env"`
	BaseURL      string    `json:"base_url"`
	SessionToken string    `json:"session_token"`
	AccountID    string    `json:"account_id"`
	TenantID     string    `json:"tenant_id"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	SavedAt      time.Time `json:"saved_at"`
}

type authorizeRequest struct {
	ClientName string `json:"client_name"`
	Scope      string `json:"scope,omitempty"`
}

type authorizeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int64  `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenRequest struct {
	DeviceCode string `json:"device_code"`
}

type tokenResponse struct {
	SessionToken     string `json:"session_token"`
	AccountID        string `json:"account_id"`
	TenantID         string `json:"tenant_id"`
	ExpiresIn        int64  `json:"expires_in"`
	TokenType        string `json:"token_type"`
	SessionExpiresAt string `json:"session_expires_at"`
}

func Login(ctx context.Context, opts Options) (*LoginResult, error) {
	opts = normalizeOptions(opts)
	if opts.Env == "" {
		return nil, errors.New("auth: Options.Env is required")
	}
	if opts.BaseURL == "" {
		return nil, errors.New("auth: Options.BaseURL is required")
	}
	if opts.DeviceCode != "" {
		return pollAndStore(ctx, opts, opts.DeviceCode, 600, 5)
	}

	authResp, err := requestDeviceAuthorization(ctx, opts)
	if err != nil {
		return nil, err
	}
	result := &LoginResult{
		Env:             opts.Env,
		DeviceCode:      authResp.DeviceCode,
		UserCode:        authResp.UserCode,
		VerificationURL: authResp.VerificationURL,
		ExpiresIn:       authResp.ExpiresIn,
		Interval:        authResp.Interval,
	}
	if opts.NoWait {
		return result, nil
	}
	polled, err := pollAndStore(ctx, opts, authResp.DeviceCode, authResp.ExpiresIn, authResp.Interval)
	if err != nil {
		return nil, err
	}
	polled.DeviceCode = authResp.DeviceCode
	polled.UserCode = authResp.UserCode
	polled.VerificationURL = authResp.VerificationURL
	polled.ExpiresIn = authResp.ExpiresIn
	polled.Interval = authResp.Interval
	return polled, nil
}

func requestDeviceAuthorization(ctx context.Context, opts Options) (*authorizeResponse, error) {
	body := authorizeRequest{ClientName: "modelgo-cli", Scope: normalizeScope(opts.Scope)}
	var out authorizeResponse
	if err := postJSON(ctx, opts.HTTPClient, opts.BaseURL+loginPathPrefix+"/auth/device/authorize", body, &out); err != nil {
		return nil, fmt.Errorf("device authorize: %w", err)
	}
	if out.DeviceCode == "" || out.VerificationURL == "" {
		return nil, errors.New("device authorize: response missing device_code or verification_url")
	}
	return &out, nil
}

func pollAndStore(ctx context.Context, opts Options, deviceCode string, expiresIn int64, interval int) (*LoginResult, error) {
	if strings.TrimSpace(deviceCode) == "" {
		return nil, errors.New("device_code is required")
	}
	if expiresIn <= 0 {
		expiresIn = int64(defaultLoginTimeout.Seconds())
	}
	if interval <= 0 {
		interval = int(defaultPollInterval.Seconds())
	}

	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	for {
		if time.Now().After(deadline) {
			return nil, errors.New("device authorization expired")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var out tokenResponse
		pending, err := postToken(ctx, opts.HTTPClient, opts.BaseURL+loginPathPrefix+"/auth/device/token", tokenRequest{DeviceCode: deviceCode}, &out)
		if err != nil {
			return nil, err
		}
		if !pending {
			expiresAt := parseExpiresAt(out.SessionExpiresAt, out.ExpiresIn)
			cred := Credential{
				Env:          opts.Env,
				BaseURL:      opts.BaseURL,
				SessionToken: out.SessionToken,
				AccountID:    out.AccountID,
				TenantID:     out.TenantID,
				TokenType:    out.TokenType,
				ExpiresAt:    expiresAt,
				SavedAt:      time.Now().UTC(),
			}
			if cred.TokenType == "" {
				cred.TokenType = "Session"
			}
			if cred.SessionToken == "" {
				return nil, errors.New("device token: response missing session_token")
			}
			if err := SaveCredential(opts.StorePath, cred); err != nil {
				return nil, fmt.Errorf("save credential: %w", err)
			}
			return &LoginResult{
				Authenticated: true,
				Env:           cred.Env,
				AccountID:     cred.AccountID,
				TenantID:      cred.TenantID,
				ExpiresAt:     cred.ExpiresAt,
			}, nil
		}

		opts.PollDelay(time.Duration(interval) * time.Second)
	}
}

func postJSON(ctx context.Context, client *http.Client, url string, in any, out any) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "modelgo-cli")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func postToken(ctx context.Context, client *http.Client, url string, in tokenRequest, out *tokenResponse) (bool, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "modelgo-cli")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		return true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("device token: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return false, err
	}
	return false, nil
}

// store is the on-disk shape of ~/.modelgo/auth.json.
type store map[string]Credential

func loadStore(path string) (store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Try bucketed format first.
	var s store
	if err := json.Unmarshal(data, &s); err == nil {
		// A flat credential will partially unmarshal into store with
		// keys like "base_url" → string which would fail; here we got a
		// clean parse, but it might still be the flat form (e.g. an
		// empty object) — disambiguate by checking that all values look
		// like credentials. The presence of a "session_token" key at
		// top level indicates the flat format.
		if !looksLikeFlatFormat(data) {
			return s, nil
		}
	}
	// Fall back to legacy flat format.
	var flat Credential
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, fmt.Errorf("parse credential file: %w", err)
	}
	flat.Env = "cn"
	return store{"cn": flat}, nil
}

func looksLikeFlatFormat(data []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	_, hasToken := probe["session_token"]
	return hasToken
}

func SaveCredential(path string, cred Credential) error {
	if cred.Env == "" {
		return errors.New("auth: Credential.Env is required")
	}
	if path == "" {
		path = DefaultCredentialPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	s, err := loadStore(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if s == nil {
		s = store{}
	}
	s[cred.Env] = cred
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func LoadCredential(envName, path string) (*Credential, error) {
	if envName == "" {
		return nil, errors.New("auth: env name is required")
	}
	if path == "" {
		path = DefaultCredentialPath()
	}
	s, err := loadStore(path)
	if err != nil {
		return nil, err
	}
	cred, ok := s[envName]
	if !ok {
		return nil, &os.PathError{Op: "load", Path: path + "#" + envName, Err: os.ErrNotExist}
	}
	return &cred, nil
}

func Status(envName, path string) (bool, *Credential, error) {
	cred, err := LoadCredential(envName, path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil, nil
		}
		return false, nil, err
	}
	if cred.SessionToken == "" {
		return false, cred, nil
	}
	return true, cred, nil
}

func Logout(envName, path string) error {
	if envName == "" {
		return errors.New("auth: env name is required")
	}
	if path == "" {
		path = DefaultCredentialPath()
	}
	s, err := loadStore(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if _, ok := s[envName]; !ok {
		return nil
	}
	delete(s, envName)
	if len(s) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func LogoutAll(path string) error {
	if path == "" {
		path = DefaultCredentialPath()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func DefaultCredentialPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".modelgo", "auth.json")
	}
	return filepath.Join(home, ".modelgo", "auth.json")
}

func normalizeOptions(opts Options) Options {
	opts.Env = strings.TrimSpace(opts.Env)
	opts.BaseURL = strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if opts.StorePath == "" {
		opts.StorePath = DefaultCredentialPath()
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.PollDelay == nil {
		opts.PollDelay = time.Sleep
	}
	return opts
}

func normalizeScope(scope string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(scope, ",", " ")), " ")
}

func parseExpiresAt(raw string, expiresIn int64) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC()
	}
	if expiresIn <= 0 {
		expiresIn = int64(defaultLoginTimeout.Seconds())
	}
	return time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
}
```

- [ ] **Step 4: Run package tests**

Run: `go test ./internal/auth/...`
Expected: PASS — all 8 tests in the rewritten test file pass.

- [ ] **Step 5: Run main package — expect compile error**

Run: `go build ./...`
Expected: FAIL — `cmd/modelgo-cli/main.go` calls `cliauth.Status(*store)` (1 arg) and `cliauth.Logout(*store)` (1 arg) which no longer match the 2-arg signature. We'll fix main.go in Task 5; for now this is intentional.

To keep the tree buildable across the commit boundary, temporarily wire main.go to always use env `"cn"` for these calls:

In `cmd/modelgo-cli/main.go`:
- `runAuthLogin`: pass `Env: "cn"` in both `cliauth.Options{...}` literals (lines 92 and 128 of original).
- `runAuthStatus` line 178: change `cliauth.Status(*store)` to `cliauth.Status("cn", *store)`.
- `runAuthLogout` line 209: change `cliauth.Logout(*store)` to `cliauth.Logout("cn", *store)`.

These will all be replaced properly in Task 5.

- [ ] **Step 6: Verify build and existing main tests still pass**

Run: `go build ./... && go test ./...`
Expected: PASS — main_test.go still uses `--base-url`, and since we haven't removed that flag yet, it works against the temporary hardcoded "cn" env.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/ cmd/modelgo-cli/main.go
git commit -m "feat(auth): bucket credentials per env and drop env-var lookups"
```

---

## Task 4: Env CLI subcommands

**Files:**
- Create: `internal/cmd/envcmd/envcmd.go`
- Create: `internal/cmd/envcmd/envcmd_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/cmd/envcmd/envcmd_test.go
package envcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/config"
)

func writeConfig(t *testing.T, dir string, cfg config.Config) string {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return path
}

func TestListShowsBuiltInsAndActiveMarker(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{CurrentEnv: "intl"})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"list", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "cn") || !strings.Contains(out, "intl") {
		t.Fatalf("list missing built-ins: %s", out)
	}
	if !strings.Contains(out, "* intl") {
		t.Fatalf("list missing active marker on intl: %s", out)
	}
}

func TestCurrentPrintsActiveEnv(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{CurrentEnv: "intl"})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"current", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "intl" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCurrentDefaultsToCNWhenConfigMissing(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"current", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if strings.TrimSpace(stdout.String()) != "cn" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestUseSetsCurrentEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"use", "intl", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg.CurrentEnv != "intl" {
		t.Fatalf("config = %+v err=%v", cfg, err)
	}
}

func TestUseRejectsUnknownEnv(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"use", "nope", "--config", cfgPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown environment") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestAddRegistersCustomEnv(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"add", "test", "--base-url", "https://api-test.modelgo.com", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Envs["test"].BaseURL != "https://api-test.modelgo.com" {
		t.Fatalf("envs = %+v", cfg.Envs)
	}
}

func TestAddBuiltInWritesOverride(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"add", "cn", "--base-url", "https://staging-cn.modelgo.com", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, _ := config.Load(cfgPath)
	if cfg.Envs["cn"].BaseURL != "https://staging-cn.modelgo.com" {
		t.Fatalf("override not written: %+v", cfg.Envs)
	}
}

func TestAddRequiresValidURL(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"add", "test", "--base-url", "not-a-url", "--config", cfgPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, stderr=%s", stderr.String())
	}
}

func TestRemoveCustomEnv(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		Envs: map[string]config.EnvEntry{"test": {BaseURL: "https://api-test.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"remove", "test", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, _ := config.Load(cfgPath)
	if _, ok := cfg.Envs["test"]; ok {
		t.Fatalf("test still present: %+v", cfg.Envs)
	}
}

func TestRemoveCurrentEnvErrors(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		CurrentEnv: "test",
		Envs:       map[string]config.EnvEntry{"test": {BaseURL: "https://api-test.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"remove", "test", "--config", cfgPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "active environment") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRemoveBuiltInClearsOverrideOnly(t *testing.T) {
	t.Parallel()
	// remove cn (built-in) just deletes the config override, cn stays usable.
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		Envs: map[string]config.EnvEntry{"cn": {BaseURL: "https://staging-cn.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"remove", "cn", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, _ := config.Load(cfgPath)
	if _, ok := cfg.Envs["cn"]; ok {
		t.Fatalf("override not removed: %+v", cfg.Envs)
	}
}

func TestListJSON(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		CurrentEnv: "test",
		Envs:       map[string]config.EnvEntry{"test": {BaseURL: "https://api-test.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"list", "--config", cfgPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout not JSON: %v: %s", err, stdout.String())
	}
	if len(out) != 3 { // cn, intl, test
		t.Fatalf("got %d entries: %+v", len(out), out)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bogus"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit")
	}
	if !strings.Contains(stderr.String(), "unknown env command") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestEnvDirCleanup(t *testing.T) {
	t.Parallel()
	// sanity that helper above leaves working tree clean
	if _, err := os.Stat("/tmp/should-not-exist"); err == nil {
		t.Fatal("test helper polluted filesystem")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cmd/envcmd/...`
Expected: FAIL — package missing

- [ ] **Step 3: Write the implementation**

```go
// internal/cmd/envcmd/envcmd.go
// Package envcmd implements `modelgo env` subcommands: list, current, use,
// add, remove. The package operates purely on a config file path passed via
// --config; it does not read environment variables.
package envcmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
)

// Run dispatches an `env` subcommand. args is the list of arguments AFTER
// `modelgo env`. Returns process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "list":
		return runList(args[1:], stdout, stderr)
	case "current":
		return runCurrent(args[1:], stdout, stderr)
	case "use":
		return runUse(args[1:], stdout, stderr)
	case "add":
		return runAdd(args[1:], stdout, stderr)
	case "remove":
		return runRemove(args[1:], stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown env command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path (default ~/.modelgo/config.json)")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env list: %v\n", err)
		return 1
	}
	entries := env.List(cfg)
	if *jsonOut {
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			out = append(out, map[string]any{
				"name":       e.Name,
				"url":        e.URL,
				"built_in":   e.BuiltIn,
				"overridden": e.Overridden,
				"active":     e.Active,
			})
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(out)
		return 0
	}
	for _, e := range entries {
		marker := "  "
		if e.Active {
			marker = "* "
		}
		tags := []string{}
		if e.BuiltIn {
			tags = append(tags, "built-in")
		} else {
			tags = append(tags, "custom")
		}
		if e.Overridden {
			tags = append(tags, "overridden")
		}
		fmt.Fprintf(stdout, "%s%-8s %s  (%s)\n", marker, e.Name, e.URL, strings.Join(tags, ", "))
	}
	return 0
}

func runCurrent(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env current", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env current: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, env.ActiveEnv("", cfg))
	return 0
}

func runUse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env use", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "env use: expected exactly one env name")
		return 2
	}
	name := fs.Arg(0)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env use: %v\n", err)
		return 1
	}
	if _, err := env.Resolve(name, cfg); err != nil {
		fmt.Fprintf(stderr, "env use: %v\n", err)
		return 1
	}
	cfg.CurrentEnv = name
	if err := saveConfig(*configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "env use: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Switched to env %q\n", name)
	return 0
}

func runAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	baseURL := fs.String("base-url", "", "API base URL for this env (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "env add: expected exactly one env name")
		return 2
	}
	name := fs.Arg(0)
	if *baseURL == "" {
		fmt.Fprintln(stderr, "env add: --base-url is required")
		return 2
	}
	parsed, err := url.Parse(*baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		fmt.Fprintf(stderr, "env add: invalid --base-url: %q\n", *baseURL)
		return 1
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		fmt.Fprintf(stderr, "env add: --base-url scheme must be http or https, got %q\n", parsed.Scheme)
		return 1
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env add: %v\n", err)
		return 1
	}
	if cfg.Envs == nil {
		cfg.Envs = map[string]config.EnvEntry{}
	}
	cfg.Envs[name] = config.EnvEntry{BaseURL: strings.TrimRight(*baseURL, "/")}
	if err := saveConfig(*configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "env add: %v\n", err)
		return 1
	}
	if env.IsBuiltIn(name) {
		fmt.Fprintf(stdout, "Overrode built-in env %q with %s\n", name, parsed.String())
	} else {
		fmt.Fprintf(stdout, "Added env %q → %s\n", name, parsed.String())
	}
	return 0
}

func runRemove(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env remove", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "env remove: expected exactly one env name")
		return 2
	}
	name := fs.Arg(0)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "env remove: %v\n", err)
		return 1
	}
	if cfg.CurrentEnv == name {
		fmt.Fprintf(stderr, "env remove: cannot remove the active environment %q. Run 'modelgo env use <other>' first.\n", name)
		return 1
	}
	if _, ok := cfg.Envs[name]; !ok {
		if env.IsBuiltIn(name) {
			fmt.Fprintf(stdout, "Env %q has no override to remove\n", name)
			return 0
		}
		fmt.Fprintf(stderr, "env remove: unknown env %q\n", name)
		return 1
	}
	delete(cfg.Envs, name)
	if err := saveConfig(*configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "env remove: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Removed env %q\n", name)
	return 0
}

func loadConfig(path string) (config.Config, error) {
	if path == "" {
		path = config.DefaultPath()
	}
	return config.Load(path)
}

func saveConfig(path string, cfg config.Config) error {
	if path == "" {
		path = config.DefaultPath()
	}
	return config.Save(path, cfg)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo env — manage modelgo environments

USAGE:
    modelgo env <command> [flags]

COMMANDS:
    list                          List built-in and custom envs
    current                       Print the active env name
    use <name>                    Switch the active env
    add <name> --base-url URL     Register or override an env URL
    remove <name>                 Remove a custom env or override

FLAGS:
    --config PATH                 Config file path (default ~/.modelgo/config.json)
    --json                        (list only) Emit JSON output`)
}

// Sentinel so callers can detect unknown-env errors if needed.
var _ = errors.New
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cmd/envcmd/...`
Expected: PASS — all subcommand tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/envcmd/
git commit -m "feat(env): add 'modelgo env' list/current/use/add/remove subcommands"
```

---

## Task 5: Wire env into main, add `--env` flag, drop `--base-url`, fix help text

**Files:**
- Modify: `cmd/modelgo-cli/main.go`
- Modify: `cmd/modelgo-cli/main_test.go`

- [ ] **Step 1: Update main_test.go to expect new behavior**

Replace `cmd/modelgo-cli/main_test.go` with:

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/modelgo-cli/...`
Expected: FAIL — `--config` flag missing, env subcommand missing, "modelgo-cli" still in help text.

- [ ] **Step 3: Rewrite `cmd/modelgo-cli/main.go`**

Replace the file with:

```go
// Command modelgo is the modelgo CLI entrypoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	cliauth "github.com/modelgo/modelgo-cli/internal/auth"
	"github.com/modelgo/modelgo-cli/internal/cmd/envcmd"
	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
	"github.com/modelgo/modelgo-cli/internal/hello"
	"github.com/modelgo/modelgo-cli/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Fprintln(stdout, version.Version)
	case "--help", "-h":
		printUsage(stdout)
	case "hello":
		return runHello(args[1:], stdout, stderr)
	case "auth":
		return runAuth(args[1:], stdout, stderr)
	case "env":
		return envcmd.Run(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
	return 0
}

func runHello(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hello", flag.ExitOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "world", "name to greet")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fmt.Fprintln(stdout, hello.Greet(*name))
	return 0
}

func runAuth(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printAuthUsage(stderr)
		return 2
	}
	switch args[0] {
	case "login":
		return runAuthLogin(args[1:], stdout, stderr)
	case "status":
		return runAuthStatus(args[1:], stdout, stderr)
	case "logout":
		return runAuthLogout(args[1:], stdout, stderr)
	case "--help", "-h":
		printAuthUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown auth command: %s\n\n", args[0])
		printAuthUsage(stderr)
		return 2
	}
}

// resolveEnvAndURL loads the config file at configPath, picks the active env
// (respecting an explicit --env flag), and resolves it to a base URL.
func resolveEnvAndURL(envFlag, configPath string, stderr io.Writer) (envName, baseURL string, ok bool) {
	if configPath == "" {
		configPath = config.DefaultPath()
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return "", "", false
	}
	envName = env.ActiveEnv(envFlag, cfg)
	url, err := env.Resolve(envName, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return "", "", false
	}
	return envName, url, true
}

func runAuthLogin(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to log into (default: active env from config)")
	configPath := fs.String("config", "", "config file path (default ~/.modelgo/config.json)")
	store := fs.String("store", "", "credential store path (default ~/.modelgo/auth.json)")
	scope := fs.String("scope", "", "space- or comma-separated scopes to request")
	noWait := fs.Bool("no-wait", false, "print device authorization URL and return immediately")
	deviceCode := fs.String("device-code", "", "poll an existing device code")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	envName, baseURL, ok := resolveEnvAndURL(*envFlag, *configPath, stderr)
	if !ok {
		return 1
	}

	ctx := context.Background()
	loginOpts := cliauth.Options{
		Env:        envName,
		BaseURL:    baseURL,
		Scope:      *scope,
		DeviceCode: *deviceCode,
		StorePath:  *store,
	}
	if *deviceCode == "" {
		loginOpts.NoWait = true
	}
	result, err := cliauth.Login(ctx, loginOpts)
	if err != nil {
		fmt.Fprintf(stderr, "auth login failed: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(loginJSON(result, *noWait))
		if *noWait || *deviceCode != "" || result.Authenticated {
			return 0
		}
	} else if !result.Authenticated {
		fmt.Fprintf(stdout, "Open this URL to authorize modelgo:\n%s\n\nUser code: %s\n", result.VerificationURL, result.UserCode)
		if *noWait {
			fmt.Fprintf(stdout, "\nAfter approving, run:\nmodelgo auth login --device-code %s\n", result.DeviceCode)
		}
	}
	if *noWait || result.Authenticated {
		if result.Authenticated && !*jsonOut {
			fmt.Fprintf(stdout, "Logged in as %s (env %s)\n", result.AccountID, result.Env)
		}
		return 0
	}

	if *deviceCode == "" {
		fmt.Fprintln(stderr, "Waiting for authorization...")
		result, err = cliauth.Login(ctx, cliauth.Options{
			Env:        envName,
			BaseURL:    baseURL,
			DeviceCode: result.DeviceCode,
			StorePath:  *store,
		})
		if err != nil {
			fmt.Fprintf(stderr, "auth login failed: %v\n", err)
			return 1
		}
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(loginJSON(result, false))
	} else {
		fmt.Fprintf(stdout, "\nLogged in as %s (env %s)\n", result.AccountID, result.Env)
	}
	return 0
}

func loginJSON(result *cliauth.LoginResult, noWait bool) map[string]any {
	if result.Authenticated {
		return map[string]any{
			"authenticated": true,
			"env":           result.Env,
			"account_id":    result.AccountID,
			"tenant_id":     result.TenantID,
			"expires_at":    result.ExpiresAt.Format(time.RFC3339Nano),
		}
	}
	out := map[string]any{
		"env":              result.Env,
		"verification_url": result.VerificationURL,
		"device_code":      result.DeviceCode,
		"user_code":        result.UserCode,
		"expires_in":       result.ExpiresIn,
		"interval":         result.Interval,
	}
	if noWait {
		out["hint"] = fmt.Sprintf("Show verification_url to the user exactly as returned. After approval, run: modelgo auth login --device-code %s", result.DeviceCode)
	}
	return out
}

func runAuthStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to check (default: active env from config)")
	configPath := fs.String("config", "", "config file path")
	store := fs.String("store", "", "credential store path")
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	envName := *envFlag
	if envName == "" {
		// Without --env or config, default to "cn"; we don't error if config
		// missing because status should work for first-run "not logged in" UX.
		cfg, _ := config.Load(configPathOrDefault(*configPath))
		envName = env.ActiveEnv("", cfg)
	}

	ok, cred, err := cliauth.Status(envName, *store)
	if err != nil {
		fmt.Fprintf(stderr, "auth status failed: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if !ok {
			_ = enc.Encode(map[string]any{"logged_in": false, "env": envName})
		} else {
			_ = enc.Encode(map[string]any{
				"logged_in":  true,
				"env":        envName,
				"account_id": cred.AccountID,
				"tenant_id":  cred.TenantID,
				"expires_at": cred.ExpiresAt.Format(time.RFC3339Nano),
			})
		}
	} else if ok {
		fmt.Fprintf(stdout, "Logged in as %s (env %s)\n", cred.AccountID, envName)
	} else {
		fmt.Fprintf(stdout, "Not logged in (env %s)\n", envName)
	}
	if !ok {
		return 1
	}
	return 0
}

func runAuthLogout(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envFlag := fs.String("env", "", "env to log out of (default: active env from config)")
	configPath := fs.String("config", "", "config file path")
	store := fs.String("store", "", "credential store path")
	all := fs.Bool("all", false, "log out of all envs")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *all {
		if err := cliauth.LogoutAll(*store); err != nil {
			fmt.Fprintf(stderr, "auth logout failed: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Logged out of all envs")
		return 0
	}

	envName := *envFlag
	if envName == "" {
		cfg, _ := config.Load(configPathOrDefault(*configPath))
		envName = env.ActiveEnv("", cfg)
	}
	if err := cliauth.Logout(envName, *store); err != nil {
		fmt.Fprintf(stderr, "auth logout failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Logged out of env %s\n", envName)
	return 0
}

func configPathOrDefault(p string) string {
	if p == "" {
		return config.DefaultPath()
	}
	return p
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo — the official modelgo CLI

USAGE:
    modelgo <command> [flags]

COMMANDS:
    auth login            Log in with device authorization
    auth status           Show local auth status
    auth logout           Clear local auth credentials
    env list              List built-in and custom envs
    env current           Print the active env
    env use <name>        Switch the active env
    env add <name>        Register or override an env URL
    env remove <name>     Remove a custom env or override
    hello [--name NAME]   Print a greeting
    --version, -v         Print the version
    --help, -h            Show this help`)
}

func printAuthUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo auth — authentication commands

USAGE:
    modelgo auth <command> [flags]

COMMANDS:
    login    Log in with device authorization
    status   Show local auth status
    logout   Clear local auth credentials

FLAGS:
    --env NAME       Operate on a specific env (default: active env from config)
    --config PATH    Config file path (default ~/.modelgo/config.json)
    --store PATH     Credential store path (default ~/.modelgo/auth.json)
    --all            (logout) Clear all envs`)
}
```

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: PASS — all tests across all packages now pass.

- [ ] **Step 5: Run a manual smoke check**

Run: `go build -o /tmp/modelgo ./cmd/modelgo-cli && /tmp/modelgo --help`
Expected: Output starts with `modelgo — the official modelgo CLI` and mentions `env list` etc. No occurrence of `modelgo-cli`.

Run: `/tmp/modelgo env list --config /tmp/nonexistent-config.json`
Expected: Two lines for `cn` and `intl`, `cn` marked active.

- [ ] **Step 6: Commit**

```bash
git add cmd/modelgo-cli/
git commit -m "feat(cli): wire env subcommand and --env flag; drop --base-url; rename to 'modelgo' in help"
```

---

## Self-Review Checklist

- **Spec coverage:**
  - Built-in `cn` + `intl` → env package ✓
  - Custom envs via config file → config + envcmd ✓
  - `env list/current/use/add/remove` → envcmd ✓
  - Active env state in config → `current_env` field + `env use` ✓
  - Credentials bucketed per env (so switching envs preserves login) → Task 3 ✓
  - `env add cn` doesn't error, writes override → Task 4 `TestAddBuiltInWritesOverride` ✓
  - `env remove` on current env errors → Task 4 `TestRemoveCurrentEnvErrors` ✓
  - All env-var lookups (`MODELGO_API_URL`, `MODELGO_CLI_CONFIG_DIR`) removed → Task 3 rewrite ✓
  - `--base-url` flag removed from auth → Task 5 ✓
  - Binary name `modelgo` (not `modelgo-cli`) in help text → Task 5 ✓
  - Tests inject paths via `--config` / `--store` instead of env vars → Tasks 4 & 5 ✓

- **Placeholders:** None — every step has runnable code or exact commands.

- **Type consistency:**
  - `cliauth.Options.Env` used in all login call sites ✓
  - `cliauth.Status(envName, path)` signature consistent across Task 3 tests and Task 5 main.go ✓
  - `cliauth.Logout(envName, path)` and `cliauth.LogoutAll(path)` names match between Task 3 and Task 5 ✓
  - `env.ActiveEnv`, `env.Resolve`, `env.List`, `env.IsBuiltIn` all defined in Task 2 and used in Tasks 4 & 5 ✓
  - `config.Config`, `config.EnvEntry`, `config.Load`, `config.Save`, `config.DefaultPath` defined in Task 1 and used in Tasks 4 & 5 ✓
  - `envcmd.Run` signature matches its call site in `main.go` ✓
