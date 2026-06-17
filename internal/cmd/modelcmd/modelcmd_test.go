package modelcmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testEnv writes a temp config pointing env "test" at srv and returns the
// common flags (--env test --config <path> --api-key mgk_test) to prepend.
func testEnv(t *testing.T, srvURL string) []string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := map[string]any{
		"current_env": "test",
		"envs":        map[string]any{"test": map[string]any{"base_url": srvURL}},
	}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MODELGO_API_KEY", "")
	return []string{"--env", "test", "--config", cfgPath, "--api-key", "mgk_test"}
}

func TestChatNonStream(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Hello there"}}]}`))
	}))
	defer srv.Close()

	args := append(testEnv(t, srv.URL), "--model", "gpt-4o", "Hi")
	var out, errOut bytes.Buffer
	if code := Chat(args, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "Hello there" {
		t.Errorf("stdout = %q", out.String())
	}
	if gotBody["model"] != "gpt-4o" {
		t.Errorf("model = %v", gotBody["model"])
	}
	if _, ok := gotBody["stream"]; ok {
		t.Errorf("stream should be absent when not requested")
	}
}

func TestChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	args := append(testEnv(t, srv.URL), "--model", "gpt-4o", "--stream", "Hi")
	var out, errOut bytes.Buffer
	if code := Chat(args, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "Hello" {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestChatPromptFromStdin(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	args := append(testEnv(t, srv.URL), "--model", "m")
	var out, errOut bytes.Buffer
	if code := Chat(args, strings.NewReader("from stdin"), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	msgs := gotBody["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)
	if last["content"] != "from stdin" {
		t.Errorf("content = %v", last["content"])
	}
}

func TestChatMissingModel(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Chat([]string{"hi"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"claude-3"}]}`))
	}))
	defer srv.Close()

	args := testEnv(t, srv.URL)
	var out, errOut bytes.Buffer
	if code := Models(args, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if out.String() != "claude-3\ngpt-4o\n" { // sorted
		t.Errorf("stdout = %q", out.String())
	}
}

func TestEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model":"emb","data":[{"embedding":[0.1,0.2,0.3]}],"usage":{"total_tokens":5}}`))
	}))
	defer srv.Close()

	args := append(testEnv(t, srv.URL), "--model", "emb", "hello world")
	var out, errOut bytes.Buffer
	if code := Embeddings(args, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "dims:    3") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestCallPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		b, _ := json.Marshal(map[string]any{"path": r.URL.Path})
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	args := append(testEnv(t, srv.URL), "/v1/images/generations", "--data", `{"prompt":"a cat"}`)
	var out, errOut bytes.Buffer
	if code := Call(args, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "/v1/images/generations") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestCallExtraHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "yes" {
			t.Errorf("X-Custom = %q", r.Header.Get("X-Custom"))
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	args := append(testEnv(t, srv.URL), "/v1/messages", "--data", `{}`, "--header", "X-Custom: yes")
	var out, errOut bytes.Buffer
	if code := Call(args, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

func TestCallMissingPath(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Call([]string{"--data", "{}"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestChatAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	args := append(testEnv(t, srv.URL), "--model", "m", "hi")
	var out, errOut bytes.Buffer
	if code := Chat(args, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "API key invalid") {
		t.Errorf("stderr = %q", errOut.String())
	}
}
