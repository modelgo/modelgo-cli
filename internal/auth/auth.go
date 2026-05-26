// Package auth implements modelgo-cli device login and local session storage.
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
	defaultBaseURL      = "https://api.modelgo.com"
	defaultPollInterval = 5 * time.Second
	defaultLoginTimeout = 10 * time.Minute

	// loginPathPrefix is the public login prefix. Login lives outside the
	// model-gateway openapi surface — modelgo-web-api owns it — but the CLI
	// hits a single public hostname (api.modelgo.com) that the deployment's
	// ingress routes by prefix. modelgo-model-gateway never sees these
	// requests; /open/v1/* is reserved for already-authenticated openapi
	// calls that carry a Bearer session_token.
	loginPathPrefix = "/v1"

	// openAPIPathPrefix is the future public prefix for already-authenticated
	// openapi calls served by model-gateway. Not used by the current command
	// set; declared here so future CLI commands (account, workspaces,
	// api-keys …) all share one constant.
	openAPIPathPrefix = "/open/v1"
)

type Options struct {
	BaseURL    string
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
	AccountID       string
	TenantID        string
	ExpiresAt       time.Time
}

type Credential struct {
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
	if opts.DeviceCode != "" {
		return pollAndStore(ctx, opts, opts.DeviceCode, 600, 5)
	}

	authResp, err := requestDeviceAuthorization(ctx, opts)
	if err != nil {
		return nil, err
	}
	result := &LoginResult{
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

func SaveCredential(path string, cred Credential) error {
	if path == "" {
		path = DefaultCredentialPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func LoadCredential(path string) (*Credential, error) {
	if path == "" {
		path = DefaultCredentialPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cred Credential
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, err
	}
	return &cred, nil
}

func Status(path string) (bool, *Credential, error) {
	cred, err := LoadCredential(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	if cred.SessionToken == "" {
		return false, cred, nil
	}
	return true, cred, nil
}

func Logout(path string) error {
	if path == "" {
		path = DefaultCredentialPath()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func DefaultCredentialPath() string {
	if dir := strings.TrimSpace(os.Getenv("MODELGO_CLI_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".modelgo", "auth.json")
	}
	return filepath.Join(home, ".modelgo", "auth.json")
}

func normalizeOptions(opts Options) Options {
	opts.BaseURL = strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if opts.BaseURL == "" {
		opts.BaseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("MODELGO_API_URL")), "/")
	}
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}
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
