package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDecodeBody_Enveloped(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","data":{"name":"test"}}`)
	var out struct {
		Name string `json:"name"`
	}
	if err := decodeBody(body, &out); err != nil {
		t.Fatalf("decodeBody: %v", err)
	}
	if out.Name != "test" {
		t.Errorf("Name = %q, want %q", out.Name, "test")
	}
}

func TestDecodeBody_EnvelopedError(t *testing.T) {
	body := []byte(`{"code":404,"msg":"tenant not found","data":null}`)
	var out struct{}
	err := decodeBody(body, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if ae.Code != 404 {
		t.Errorf("Code = %d, want 404", ae.Code)
	}
	if ae.Message != "tenant not found" {
		t.Errorf("Message = %q, want %q", ae.Message, "tenant not found")
	}
}

func TestDecodeBody_Flat(t *testing.T) {
	body := []byte(`{"name":"flat"}`)
	var out struct {
		Name string `json:"name"`
	}
	if err := decodeBody(body, &out); err != nil {
		t.Fatalf("decodeBody: %v", err)
	}
	if out.Name != "flat" {
		t.Errorf("Name = %q, want %q", out.Name, "flat")
	}
}

func TestDecodeBody_NullData(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","data":null}`)
	var out struct{}
	if err := decodeBody(body, &out); err != nil {
		t.Fatalf("decodeBody: %v", err)
	}
}

func TestClient_Get(t *testing.T) {
	type resp struct {
		Value string `json:"value"`
	}
	want := resp{Value: "hello"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok123" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer tok123")
		}
		if r.URL.Path != "/open/v1/test/path" {
			t.Errorf("Path = %q, want %q", r.URL.Path, "/open/v1/test/path")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok", "data": want,
		})
	}))
	defer srv.Close()

	c := &Client{
		HTTPClient:   srv.Client(),
		BaseURL:      srv.URL,
		SessionToken: "tok123",
	}
	var got resp
	if err := c.Get(context.Background(), "test/path", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClient_GetWithQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "10")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok", "data": []any{},
		})
	}))
	defer srv.Close()

	c := &Client{
		HTTPClient:   srv.Client(),
		BaseURL:      srv.URL,
		SessionToken: "tok123",
	}
	var got []any
	params := url.Values{"limit": {"10"}}
	if err := c.GetWithQuery(context.Background(), "items", params, &got); err != nil {
		t.Fatalf("GetWithQuery: %v", err)
	}
}

func TestClient_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{
		HTTPClient:   srv.Client(),
		BaseURL:      srv.URL,
		SessionToken: "expired",
	}
	var got any
	err := c.Get(context.Background(), "test", &got)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !contains(err.Error(), "session expired") {
		t.Errorf("error = %q, want something about session expired", err.Error())
	}
}

func TestClient_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := &Client{
		HTTPClient:   srv.Client(),
		BaseURL:      srv.URL,
		SessionToken: "tok",
	}
	var got any
	err := c.Get(context.Background(), "test", &got)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !contains(err.Error(), "permission denied") {
		t.Errorf("error = %q, want something about permission denied", err.Error())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || searchContains(s, sub))
}

func searchContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
