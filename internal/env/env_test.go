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
	if url != "https://api.modelgo.ai" {
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
	if !ok || !intl.BuiltIn || intl.Overridden || intl.URL != "https://api.modelgo.ai" {
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
