// Package modelapi provides an HTTP client for the modelgo gateway's
// OpenAI-compatible model data plane (/v1/*). Unlike internal/apiclient — which
// drives the /open/v1/* control plane with a device-login session token — these
// requests authenticate with a model API key (mgk_...) via the
// `Authorization: Bearer` header, exactly like calling OpenAI/Anthropic
// directly. The gateway reverse-proxies to the upstream provider.
package modelapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelgo/modelgo-cli/internal/config"
	"github.com/modelgo/modelgo-cli/internal/env"
)

// EnvAPIKey is the environment variable consulted for the model API key when no
// explicit --api-key flag is given.
const EnvAPIKey = "MODELGO_API_KEY"

const defaultTimeout = 5 * time.Minute // model calls (esp. streaming) run long

// Client calls the gateway's /v1/* model endpoints with an API key.
type Client struct {
	HTTPClient *http.Client
	Env        string
	BaseURL    string // trimmed of trailing slash; no /v1 suffix
	APIKey     string
}

// Params bundles the inputs needed to build a Client. APIKey, when empty, is
// resolved from the MODELGO_API_KEY env var or the stored per-env key.
type Params struct {
	APIKeyFlag string // value of --api-key, or ""
	EnvFlag    string // value of --env, or ""
	ConfigPath string // override config path, or ""
	HTTPClient *http.Client
}

// New builds a Client by resolving the env base URL and the API key.
func New(p Params) (*Client, error) {
	cfgPath := p.ConfigPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	envName := env.ActiveEnv(p.EnvFlag, cfg)
	baseURL, err := env.Resolve(envName, cfg)
	if err != nil {
		return nil, err
	}
	key, err := ResolveAPIKey(p.APIKeyFlag, envName, cfg)
	if err != nil {
		return nil, err
	}
	httpClient := p.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		HTTPClient: httpClient,
		Env:        envName,
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     key,
	}, nil
}

// ResolveAPIKey returns the model API key using the precedence
// flag > MODELGO_API_KEY env var > stored per-env key in config.
func ResolveAPIKey(flagVal, envName string, cfg config.Config) (string, error) {
	if v := strings.TrimSpace(flagVal); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv(EnvAPIKey)); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(cfg.APIKeys[envName]); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("no API key for env %q. Pass --api-key, set %s, or run `modelgo key set`", envName, EnvAPIKey)
}

// Do sends an authenticated request to a /v1/* path (e.g. "/v1/chat/completions").
// body may be nil. The caller owns the returned response body and must close it.
// Non-2xx responses are returned as an *HTTPError (body already drained), so
// streaming callers can branch on status before reading the stream.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	u := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		u = c.BaseURL + "/" + strings.TrimPrefix(path, "/")
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("User-Agent", "modelgo")
	if _, ok := headers["Content-Type"]; !ok && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach server at %s: %w", c.BaseURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
	}
	return resp, nil
}

// HTTPError is a non-2xx response from the gateway/upstream. Body is the raw
// (usually JSON) error payload, useful for surfacing the provider's message.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return "API key invalid or expired (HTTP 401). Check your key or run `modelgo key set`"
	case http.StatusPaymentRequired:
		return "payment required (HTTP 402). Top up with `modelgo balance` or use `modelgo pay` for x402"
	case http.StatusForbidden:
		return "permission denied (HTTP 403). Check your access with `modelgo permissions`"
	case http.StatusTooManyRequests:
		return "rate limited (HTTP 429). Slow down and retry"
	}
	if e.Body != "" {
		return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}
