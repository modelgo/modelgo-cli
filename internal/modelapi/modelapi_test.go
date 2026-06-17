package modelapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/config"
)

func TestResolveAPIKeyPrecedence(t *testing.T) {
	cfg := config.Config{APIKeys: map[string]string{"cn": "mgk_stored"}}

	t.Setenv(EnvAPIKey, "")
	got, err := ResolveAPIKey("mgk_flag", "cn", cfg)
	if err != nil || got != "mgk_flag" {
		t.Fatalf("flag should win: got %q err %v", got, err)
	}

	t.Setenv(EnvAPIKey, "mgk_env")
	got, err = ResolveAPIKey("", "cn", cfg)
	if err != nil || got != "mgk_env" {
		t.Fatalf("env should beat stored: got %q err %v", got, err)
	}

	t.Setenv(EnvAPIKey, "")
	got, err = ResolveAPIKey("", "cn", cfg)
	if err != nil || got != "mgk_stored" {
		t.Fatalf("stored should be used: got %q err %v", got, err)
	}

	if _, err = ResolveAPIKey("", "intl", cfg); err == nil {
		t.Fatal("expected error when no key resolvable")
	}
}

func TestDoSendsBearerAndBody(t *testing.T) {
	var gotAuth, gotCT, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "mgk_test"}
	resp, err := c.Do(context.Background(), http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x"}`), nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer mgk_test" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q", gotCT)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody != `{"model":"x"}` {
		t.Errorf("body = %q", gotBody)
	}
}

func TestDoNon2xxReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "mgk_test"}
	_, err := c.Do(context.Background(), http.MethodGet, "/v1/models", nil, nil)
	var he *HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("expected *HTTPError, got %v", err)
	}
	if he.Status != 401 {
		t.Errorf("status = %d", he.Status)
	}
	if !strings.Contains(he.Error(), "API key invalid") {
		t.Errorf("error message = %q", he.Error())
	}
	if !strings.Contains(he.Body, "bad key") {
		t.Errorf("body = %q", he.Body)
	}
}
