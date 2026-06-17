package keycmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/config"
)

func TestKeySetShowRemove(t *testing.T) {
	t.Setenv("MODELGO_API_KEY", "")
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	base := []string{"--env", "cn", "--config", cfgPath}

	// set
	var out, errOut bytes.Buffer
	if code := Run(append([]string{"set"}, append(base, "mgk_secret_key_12345")...), strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("set exit %d: %s", code, errOut.String())
	}
	cfg, _ := config.Load(cfgPath)
	if cfg.APIKeys["cn"] != "mgk_secret_key_12345" {
		t.Fatalf("stored key = %q", cfg.APIKeys["cn"])
	}

	// show (masked, no full secret)
	out.Reset()
	errOut.Reset()
	if code := Run(append([]string{"show"}, base...), strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("show exit %d: %s", code, errOut.String())
	}
	if strings.Contains(out.String(), "secret_key") {
		t.Errorf("show leaked the secret: %q", out.String())
	}
	if !strings.Contains(out.String(), "mgk_secr") {
		t.Errorf("show should print masked prefix: %q", out.String())
	}

	// remove
	out.Reset()
	errOut.Reset()
	if code := Run(append([]string{"remove"}, base...), strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("remove exit %d: %s", code, errOut.String())
	}
	cfg, _ = config.Load(cfgPath)
	if _, ok := cfg.APIKeys["cn"]; ok {
		t.Errorf("key not removed")
	}
}

// Regression: flags must be honored even when the positional KEY comes first.
func TestKeySetFlagsAfterKey(t *testing.T) {
	t.Setenv("MODELGO_API_KEY", "")
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	var out, errOut bytes.Buffer
	code := Run([]string{"set", "mgk_key_first_abcdef", "--env", "intl", "--config", cfgPath}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	cfg, _ := config.Load(cfgPath)
	if cfg.APIKeys["intl"] != "mgk_key_first_abcdef" {
		t.Fatalf("expected key under intl, got %v", cfg.APIKeys)
	}
	if _, ok := cfg.APIKeys["cn"]; ok {
		t.Errorf("must not write default env when --env given: %v", cfg.APIKeys)
	}
}

func TestKeySetFromStdin(t *testing.T) {
	t.Setenv("MODELGO_API_KEY", "")
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	var out, errOut bytes.Buffer
	code := Run([]string{"set", "--env", "cn", "--config", cfgPath}, strings.NewReader("mgk_from_stdin_abcdef\n"), &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	cfg, _ := config.Load(cfgPath)
	if cfg.APIKeys["cn"] != "mgk_from_stdin_abcdef" {
		t.Errorf("stored = %q", cfg.APIKeys["cn"])
	}
}

func TestKeyShowNone(t *testing.T) {
	t.Setenv("MODELGO_API_KEY", "")
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	var out, errOut bytes.Buffer
	if code := Run([]string{"show", "--env", "cn", "--config", cfgPath}, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Fatalf("expected exit 1 when no key, got %d", code)
	}
}
