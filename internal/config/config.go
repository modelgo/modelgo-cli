// Package config reads and writes the modelgo-cli user config file at
// ~/.modelgo/config.json. The config file is the sole source of user-level
// settings (current env, custom env definitions). No environment variables
// are consulted.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// EnvEntry is a user-defined environment override (typically pointing at a
// custom or internal-test API gateway). Setting an entry for a built-in env
// name (e.g. "cn") overrides that built-in's URL.
type EnvEntry struct {
	BaseURL string `json:"base_url"`
}

// Config is the on-disk shape of ~/.modelgo/config.json.
type Config struct {
	CurrentEnv string              `json:"current_env,omitempty"`
	Envs       map[string]EnvEntry `json:"envs,omitempty"`
}

// Load reads the config file at path. A missing file returns an empty
// Config and nil error — first-run users have no config.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Envs == nil {
		cfg.Envs = map[string]EnvEntry{}
	}
	return cfg, nil
}

// Save writes the config file at path with 0600 permissions, creating
// parent directories as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// DefaultPath returns the canonical config file path (~/.modelgo/config.json).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".modelgo", "config.json")
	}
	return filepath.Join(home, ".modelgo", "config.json")
}
