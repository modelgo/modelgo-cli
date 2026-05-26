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
