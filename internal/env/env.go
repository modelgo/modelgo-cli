// Package env defines built-in modelgo environments and resolves env names
// to API base URLs. Resolution order: config override > built-in > error.
package env

import (
	"errors"
	"fmt"
	"sort"

	"github.com/modelgo/modelgo-cli/internal/config"
)

const DefaultEnv = "cn"

var builtIn = map[string]string{
	"cn":   "https://api.modelgo.com",
	"intl": "https://api.modelgo.global",
}

// ErrUnknownEnv is returned when an env name is neither built-in nor present
// in the config file's envs map.
var ErrUnknownEnv = errors.New("unknown environment")

// Resolve returns the base URL for the named env. Config overrides win over
// built-in URLs so users can repoint cn/intl at a staging host via
// `modelgo env add cn --base-url ...`.
func Resolve(name string, cfg config.Config) (string, error) {
	if entry, ok := cfg.Envs[name]; ok && entry.BaseURL != "" {
		return entry.BaseURL, nil
	}
	if url, ok := builtIn[name]; ok {
		return url, nil
	}
	return "", fmt.Errorf("%w: %q. Run 'modelgo env add %s --base-url URL' to register it", ErrUnknownEnv, name, name)
}

// ActiveEnv picks the env name to operate on given an explicit value
// (typically from a --env flag) and the loaded config.
func ActiveEnv(explicit string, cfg config.Config) string {
	if explicit != "" {
		return explicit
	}
	if cfg.CurrentEnv != "" {
		return cfg.CurrentEnv
	}
	return DefaultEnv
}

// IsBuiltIn reports whether name is a built-in env (cn, intl).
func IsBuiltIn(name string) bool {
	_, ok := builtIn[name]
	return ok
}

// Entry is a row in `modelgo env list`.
type Entry struct {
	Name       string
	URL        string
	BuiltIn    bool
	Overridden bool // built-in whose URL is shadowed by a config entry
	Active     bool
}

// List returns all known envs (built-ins plus custom from config) sorted by
// name, with the active flag set on the current env.
func List(cfg config.Config) []Entry {
	seen := map[string]Entry{}
	for name, url := range builtIn {
		seen[name] = Entry{Name: name, URL: url, BuiltIn: true}
	}
	for name, entry := range cfg.Envs {
		if entry.BaseURL == "" {
			continue
		}
		e := seen[name]
		e.Name = name
		e.URL = entry.BaseURL
		if e.BuiltIn {
			e.Overridden = true
		}
		seen[name] = e
	}
	active := ActiveEnv("", cfg)
	out := make([]Entry, 0, len(seen))
	for _, e := range seen {
		e.Active = e.Name == active
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
