// Package apiclient provides a shared HTTP client for authenticated API calls
// through the modelgo gateway. All requests go via /open/v1/* with a Bearer
// session_token; the gateway reverse-proxies to the appropriate backend
// (permissions, billing, observer).
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelgo/modelgo-cli/internal/auth"
	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
)

const (
	openAPIPathPrefix = "/open/v1"
	defaultTimeout    = 30 * time.Second
)

// APIError represents a business error returned by the gateway or backend.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (code %d)", e.Message, e.Code)
}

// IsAPIError checks whether an error is an *APIError.
func IsAPIError(err error) bool {
	var ae *APIError
	return errors.As(err, &ae)
}

// Client wraps an HTTP client that sends authenticated requests through the
// modelgo gateway. It resolves the env, base URL, and active tenant credential
// from disk at construction time.
type Client struct {
	HTTPClient   *http.Client
	Env          string
	BaseURL      string
	SessionToken string
	TenantID     string
	TenantSlug   string
}

// Option configures a Client.
type Option func(*clientConfig)

// clientConfig holds the configurable inputs for NewFromConfig.
type clientConfig struct {
	httpClient    *http.Client
	configPath    string
	storePath     string
	envFlag       string
	tenantOverride string
}

// WithHTTPClient sets a custom HTTP client (useful for testing).
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *clientConfig) { cfg.httpClient = c }
}

// WithConfigPath overrides the config file path (default ~/.modelgo/config.json).
func WithConfigPath(p string) Option {
	return func(cfg *clientConfig) { cfg.configPath = p }
}

// WithStorePath overrides the credential store path (default ~/.modelgo/auth.json).
func WithStorePath(p string) Option {
	return func(cfg *clientConfig) { cfg.storePath = p }
}

// WithEnv overrides the active env.
func WithEnv(e string) Option {
	return func(cfg *clientConfig) { cfg.envFlag = e }
}

// NewFromConfig builds a Client by reading the config file and credential store.
// If tenantOverride is non-empty it selects that tenant (by slug or id) instead
// of the env's active pointer.
func NewFromConfig(tenantOverride string, opts ...Option) (*Client, error) {
	cc := &clientConfig{tenantOverride: tenantOverride}
	for _, o := range opts {
		o(cc)
	}

	cfgPath := cc.configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	envName := env.ActiveEnv(cc.envFlag, cfg)
	baseURL, err := env.Resolve(envName, cfg)
	if err != nil {
		return nil, err
	}

	storePath := cc.storePath
	if storePath == "" {
		storePath = auth.DefaultCredentialPath()
	}
	cred, err := auth.ResolveActiveOrFlag(envName, tenantOverride, storePath)
	if err != nil {
		// With an explicit --tenant override, surface the specific resolution
		// error (e.g. `tenant "x" not logged in for env "cn"`) so the user fixes
		// the tenant rather than being told to re-authenticate.
		if tenantOverride == "" {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("not logged in. Run `modelgo auth login` first")
			}
			// auth.ResolveActiveOrFlag can also return descriptive errors for
			// missing tenants; treat those the same way.
			if strings.Contains(err.Error(), "not logged in") || strings.Contains(err.Error(), "no active") {
				return nil, fmt.Errorf("not logged in. Run `modelgo auth login` first")
			}
		}
		return nil, err
	}
	if cred.SessionToken == "" {
		return nil, fmt.Errorf("not logged in. Run `modelgo auth login` first")
	}

	c := &Client{
		HTTPClient:   &http.Client{Timeout: defaultTimeout},
		Env:          envName,
		BaseURL:      strings.TrimRight(baseURL, "/"),
		SessionToken: cred.SessionToken,
		TenantID:     cred.TenantID,
		TenantSlug:   cred.TenantSlug,
	}
	if cc.httpClient != nil {
		c.HTTPClient = cc.httpClient
	}
	return c, nil
}

// Get sends an authenticated GET request to path (relative to /open/v1/) and
// decodes the JSON response (transparently unwrapping the {code,msg,data}
// envelope) into result.
func (c *Client) Get(ctx context.Context, path string, result any) error {
	return c.GetWithQuery(ctx, path, nil, result)
}

// GetWithQuery is like Get but appends query parameters.
func (c *Client) GetWithQuery(ctx context.Context, path string, params url.Values, result any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, params, nil)
	if err != nil {
		return err
	}
	return c.do(req, result)
}

func (c *Client) newRequest(ctx context.Context, method, path string, params url.Values, body io.Reader) (*http.Request, error) {
	u := c.BaseURL + openAPIPathPrefix + "/" + strings.TrimPrefix(path, "/")
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.SessionToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "modelgo")
	return req, nil
}

// apiEnvelope is the gateway's standard wrapper: {"code":0,"msg":"ok","data":{...}}.
type apiEnvelope struct {
	Code *int            `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *Client) do(req *http.Request, result any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach server at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("session expired. Run `modelgo auth login` to re-authenticate")
	}
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("permission denied. Check your access with `modelgo permissions`")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return decodeBody(body, result)
}

// decodeBody unmarshals an API response body into out, transparently unwrapping
// the {code,msg,data} envelope when present. A non-zero code is treated as a
// business error carrying msg. Flat (un-enveloped) bodies decode whole.
func decodeBody(body []byte, out any) error {
	var env apiEnvelope
	enveloped := false
	if err := json.Unmarshal(body, &env); err == nil && (env.Code != nil || env.Data != nil) {
		enveloped = true
	}
	if !enveloped {
		return json.Unmarshal(body, out)
	}
	if env.Code != nil && *env.Code != 0 {
		msg := env.Msg
		if msg == "" {
			msg = "request failed"
		}
		return &APIError{Code: *env.Code, Message: msg}
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}
