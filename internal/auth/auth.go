// Package auth implements modelgo-cli device login and multi-tenant credential
// storage. ~/.modelgo/auth.json holds a JSON object keyed by env name. Each env
// is a bucket holding every tenant credential the user has logged into for that
// env plus an active pointer:
//
//	{
//	  "cn": {
//	    "env": "cn",
//	    "base_url": "https://api.modelgo.com",
//	    "active_tenant_id": "ten_2",
//	    "previous_tenant_id": "ten_1",
//	    "tenants": { "ten_1": {Credential}, "ten_2": {Credential} }
//	  }
//	}
//
// Multiple tenants coexist; switching the active tenant (`tenant use`) is a
// pointer flip and never requires re-login. There is no backward-compatible
// migration from older flat or single-credential-per-env layouts.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	TenantSlug   string    `json:"tenant_slug,omitempty"`
	TenantName   string    `json:"tenant_name,omitempty"`
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
	body := authorizeRequest{ClientName: "modelgo", Scope: normalizeScope(opts.Scope)}
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
	req.Header.Set("User-Agent", "modelgo")
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
	req.Header.Set("User-Agent", "modelgo")
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

// envBucket holds all tenant credentials for one env plus the active pointer.
type envBucket struct {
	Env              string                `json:"env"`
	BaseURL          string                `json:"base_url"`
	ActiveTenantID   string                `json:"active_tenant_id"`
	PreviousTenantID string                `json:"previous_tenant_id,omitempty"`
	Tenants          map[string]Credential `json:"tenants"`
}

// store is the on-disk shape of ~/.modelgo/auth.json, keyed by env name.
type store map[string]*envBucket

func loadStore(path string) (store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse credential file (run `modelgo auth login` to re-create): %w", err)
	}
	return s, nil
}

func writeStore(path string, s store) error {
	if path == "" {
		path = DefaultCredentialPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func SaveCredential(path string, cred Credential) error {
	if cred.Env == "" {
		return errors.New("auth: Credential.Env is required")
	}
	if cred.TenantID == "" {
		return errors.New("auth: Credential.TenantID is required")
	}
	if path == "" {
		path = DefaultCredentialPath()
	}
	s, err := loadStore(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		s = store{}
	}
	b := s[cred.Env]
	if b == nil {
		b = &envBucket{Env: cred.Env, BaseURL: cred.BaseURL, Tenants: map[string]Credential{}}
		s[cred.Env] = b
	}
	if b.Tenants == nil {
		b.Tenants = map[string]Credential{}
	}
	b.Env = cred.Env
	b.BaseURL = cred.BaseURL
	b.Tenants[cred.TenantID] = cred
	if b.ActiveTenantID != cred.TenantID {
		b.PreviousTenantID = b.ActiveTenantID
		b.ActiveTenantID = cred.TenantID
	}
	return writeStore(path, s)
}

func LoadCredential(envName, tenantID, path string) (*Credential, error) {
	if envName == "" || tenantID == "" {
		return nil, errors.New("auth: env and tenant are required")
	}
	if path == "" {
		path = DefaultCredentialPath()
	}
	s, err := loadStore(path)
	if err != nil {
		return nil, err
	}
	b := s[envName]
	if b == nil {
		return nil, &os.PathError{Op: "load", Path: path + "#" + envName, Err: os.ErrNotExist}
	}
	cred, ok := b.Tenants[tenantID]
	if !ok {
		return nil, &os.PathError{Op: "load", Path: path + "#" + envName + "/" + tenantID, Err: os.ErrNotExist}
	}
	return &cred, nil
}

// LoadActive returns the credential the env's active pointer references.
func LoadActive(envName, path string) (*Credential, error) {
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
	b := s[envName]
	if b == nil || b.ActiveTenantID == "" {
		return nil, &os.PathError{Op: "load-active", Path: path + "#" + envName, Err: os.ErrNotExist}
	}
	cred := b.Tenants[b.ActiveTenantID]
	return &cred, nil
}

// ListTenants returns all logged-in credentials for an env plus the active id.
// A missing store is not an error: it returns (nil, "", nil).
func ListTenants(envName, path string) ([]Credential, string, error) {
	if path == "" {
		path = DefaultCredentialPath()
	}
	s, err := loadStore(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", err
	}
	b := s[envName]
	if b == nil {
		return nil, "", nil
	}
	out := make([]Credential, 0, len(b.Tenants))
	for _, c := range b.Tenants {
		out = append(out, c)
	}
	return out, b.ActiveTenantID, nil
}

// UseTenant flips the env's active pointer to tenantID, recording the prior
// active tenant as previous. Errors if the tenant is not logged in.
func UseTenant(envName, tenantID, path string) error {
	if path == "" {
		path = DefaultCredentialPath()
	}
	s, err := loadStore(path)
	if err != nil {
		return err
	}
	b := s[envName]
	if b == nil {
		return fmt.Errorf("env %q has no logged-in tenants; run `modelgo auth login`", envName)
	}
	if _, ok := b.Tenants[tenantID]; !ok {
		return fmt.Errorf("tenant %q not logged in for env %q; run `modelgo auth login` to add it", tenantID, envName)
	}
	if b.ActiveTenantID != tenantID {
		b.PreviousTenantID = b.ActiveTenantID
		b.ActiveTenantID = tenantID
	}
	return writeStore(path, s)
}

// UsePreviousTenant switches the active pointer back to the previous tenant.
func UsePreviousTenant(envName, path string) error {
	if path == "" {
		path = DefaultCredentialPath()
	}
	s, err := loadStore(path)
	if err != nil {
		return err
	}
	b := s[envName]
	if b == nil || b.PreviousTenantID == "" {
		return fmt.Errorf("no previous tenant to switch back to for env %q", envName)
	}
	return UseTenant(envName, b.PreviousTenantID, path)
}

// ResolveTenantID maps a slug-or-id to a logged-in tenant id within an env.
func ResolveTenantID(envName, slugOrID, path string) (string, error) {
	creds, _, err := ListTenants(envName, path)
	if err != nil {
		return "", err
	}
	for _, c := range creds {
		if c.TenantID == slugOrID || c.TenantSlug == slugOrID {
			return c.TenantID, nil
		}
	}
	return "", fmt.Errorf("tenant %q not logged in for env %q", slugOrID, envName)
}

// ResolveActiveOrFlag resolves the credential a command should use: when
// flagTenant is non-empty it overrides the active pointer (slug-or-id), else
// the env's active tenant is used. It never mutates the active pointer.
func ResolveActiveOrFlag(envName, flagTenant, path string) (*Credential, error) {
	if strings.TrimSpace(flagTenant) == "" {
		return LoadActive(envName, path)
	}
	tenantID, err := ResolveTenantID(envName, strings.TrimSpace(flagTenant), path)
	if err != nil {
		return nil, err
	}
	return LoadCredential(envName, tenantID, path)
}

func Status(envName, path string) (bool, *Credential, error) {
	cred, err := LoadActive(envName, path)
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

// Logout removes a single tenant from an env when tenantID is non-empty, or the
// whole env bucket when tenantID is empty. The file is removed when it becomes
// empty.
func Logout(envName, tenantID, path string) error {
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
	b := s[envName]
	if b == nil {
		return nil
	}
	if tenantID == "" { // clear the whole env
		delete(s, envName)
	} else {
		delete(b.Tenants, tenantID)
		if b.PreviousTenantID == tenantID {
			b.PreviousTenantID = ""
		}
		if b.ActiveTenantID == tenantID {
			b.ActiveTenantID = ""
			for id := range b.Tenants { // fall back to any remaining tenant
				b.ActiveTenantID = id
				break
			}
		}
		if len(b.Tenants) == 0 {
			delete(s, envName)
		}
	}
	if len(s) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeStore(path, s)
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

// RemoteTenant is one tenant the account belongs to, as returned by the server.
type RemoteTenant struct {
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Role      string `json:"role"`
	IsDefault bool   `json:"is_default"`
}

// FetchTenants lists every tenant the account behind the given credential
// belongs to, by calling GET {BaseURL}/open/v1/tenants with the credential's
// session token as a Bearer token. The route is served by model-gateway, which
// reverse-proxies to web-api/permissions. The response envelope is
// {"data": [...]}; a bare array is also accepted.
func FetchTenants(ctx context.Context, client *http.Client, cred *Credential) ([]RemoteTenant, error) {
	if cred == nil || cred.SessionToken == "" {
		return nil, errors.New("auth: a logged-in session is required to fetch tenants")
	}
	if cred.BaseURL == "" {
		return nil, errors.New("auth: credential has no base_url")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	url := strings.TrimRight(cred.BaseURL, "/") + openAPIPathPrefix + "/tenants"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cred.SessionToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "modelgo")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list tenants: HTTP %d", resp.StatusCode)
	}
	// Accept either {"data":[...]} or a bare [...].
	var enveloped struct {
		Data []RemoteTenant `json:"data"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &enveloped); err == nil && enveloped.Data != nil {
		return enveloped.Data, nil
	}
	var bare []RemoteTenant
	if err := json.Unmarshal(body, &bare); err != nil {
		return nil, fmt.Errorf("list tenants: decode response: %w", err)
	}
	return bare, nil
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
