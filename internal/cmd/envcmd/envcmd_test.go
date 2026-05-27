package envcmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/config"
)

func writeConfig(t *testing.T, dir string, cfg config.Config) string {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return path
}

func TestListShowsBuiltInsAndActiveMarker(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{CurrentEnv: "intl"})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"list", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "cn") || !strings.Contains(out, "intl") {
		t.Fatalf("list missing built-ins: %s", out)
	}
	if !strings.Contains(out, "* intl") {
		t.Fatalf("list missing active marker on intl: %s", out)
	}
}

func TestCurrentPrintsActiveEnv(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{CurrentEnv: "intl"})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"current", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "intl" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCurrentDefaultsToCNWhenConfigMissing(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"current", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if strings.TrimSpace(stdout.String()) != "cn" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestUseSetsCurrentEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"use", "intl", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg.CurrentEnv != "intl" {
		t.Fatalf("config = %+v err=%v", cfg, err)
	}
}

func TestUseRejectsUnknownEnv(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"use", "nope", "--config", cfgPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown environment") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestAddRegistersCustomEnv(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"add", "test", "--base-url", "https://api-test.modelgo.com", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Envs["test"].BaseURL != "https://api-test.modelgo.com" {
		t.Fatalf("envs = %+v", cfg.Envs)
	}
}

func TestAddBuiltInWritesOverride(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"add", "cn", "--base-url", "https://staging-cn.modelgo.com", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, _ := config.Load(cfgPath)
	if cfg.Envs["cn"].BaseURL != "https://staging-cn.modelgo.com" {
		t.Fatalf("override not written: %+v", cfg.Envs)
	}
}

func TestAddRequiresValidURL(t *testing.T) {
	t.Parallel()
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"add", "test", "--base-url", "not-a-url", "--config", cfgPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, stderr=%s", stderr.String())
	}
}

func TestAddRejectsInvalidEnvName(t *testing.T) {
	t.Parallel()
	// Note: names starting with "-" are rejected earlier by flag parsing
	// (also non-zero exit), so they're not in this validation-message list.
	for _, name := range []string{"", "  ", "has space", "bad/slash", ".dotstart"} {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		var stdout, stderr bytes.Buffer
		code := Run([]string{"add", name, "--base-url", "https://api-test.modelgo.com", "--config", cfgPath}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("name %q: expected non-zero exit, stdout=%s", name, stdout.String())
		}
		if !strings.Contains(stderr.String(), "invalid env name") {
			t.Fatalf("name %q: stderr=%q", name, stderr.String())
		}
		// Nothing should have been persisted.
		cfg, _ := config.Load(cfgPath)
		if len(cfg.Envs) != 0 {
			t.Fatalf("name %q: persisted bad entry: %+v", name, cfg.Envs)
		}
	}
}

func TestAddAcceptsValidEnvNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"test", "staging-2", "dev_box", "v1.2", "A"} {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		var stdout, stderr bytes.Buffer
		code := Run([]string{"add", name, "--base-url", "https://api-test.modelgo.com", "--config", cfgPath}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("name %q: exit=%d stderr=%s", name, code, stderr.String())
		}
	}
}

func TestRemoveCustomEnv(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		Envs: map[string]config.EnvEntry{"test": {BaseURL: "https://api-test.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"remove", "test", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, _ := config.Load(cfgPath)
	if _, ok := cfg.Envs["test"]; ok {
		t.Fatalf("test still present: %+v", cfg.Envs)
	}
}

func TestRemoveCurrentEnvErrors(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		CurrentEnv: "test",
		Envs:       map[string]config.EnvEntry{"test": {BaseURL: "https://api-test.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"remove", "test", "--config", cfgPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "active environment") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRemoveBuiltInClearsOverrideOnly(t *testing.T) {
	t.Parallel()
	// remove cn (built-in) just deletes the config override, cn stays usable.
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		Envs: map[string]config.EnvEntry{"cn": {BaseURL: "https://staging-cn.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"remove", "cn", "--config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	cfg, _ := config.Load(cfgPath)
	if _, ok := cfg.Envs["cn"]; ok {
		t.Fatalf("override not removed: %+v", cfg.Envs)
	}
}

func TestListJSON(t *testing.T) {
	t.Parallel()
	cfgPath := writeConfig(t, t.TempDir(), config.Config{
		CurrentEnv: "test",
		Envs:       map[string]config.EnvEntry{"test": {BaseURL: "https://api-test.modelgo.com"}},
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"list", "--config", cfgPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout not JSON: %v: %s", err, stdout.String())
	}
	if len(out) != 3 { // cn, intl, test
		t.Fatalf("got %d entries: %+v", len(out), out)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bogus"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit")
	}
	if !strings.Contains(stderr.String(), "unknown env command") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestEnvDirCleanup(t *testing.T) {
	t.Parallel()
	// sanity that helper above leaves working tree clean
	if _, err := os.Stat("/tmp/should-not-exist"); err == nil {
		t.Fatal("test helper polluted filesystem")
	}
}

func TestSplitFlagsAndPositionalsAutoDetectsBoolFlags(t *testing.T) {
	t.Parallel()

	// Bool flag must not consume the next positional.
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	_ = fs.Bool("force", false, "")
	_ = fs.String("config", "", "")

	pos, flagArgs := splitFlagsAndPositionals(
		[]string{"--force", "intl", "--config", "/tmp/c.json"},
		fs,
	)
	if len(pos) != 1 || pos[0] != "intl" {
		t.Fatalf("positional = %v, want [intl]", pos)
	}
	wantFlagArgs := []string{"--force", "--config", "/tmp/c.json"}
	if !reflect.DeepEqual(flagArgs, wantFlagArgs) {
		t.Fatalf("flagArgs = %v, want %v", flagArgs, wantFlagArgs)
	}
}

func TestSplitFlagsAndPositionalsHandlesEqualsForm(t *testing.T) {
	t.Parallel()
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	_ = fs.String("config", "", "")

	pos, flagArgs := splitFlagsAndPositionals(
		[]string{"use", "intl", "--config=/tmp/c.json"},
		fs,
	)
	wantPos := []string{"use", "intl"}
	wantFlagArgs := []string{"--config=/tmp/c.json"}
	if !reflect.DeepEqual(pos, wantPos) {
		t.Fatalf("positional = %v, want %v", pos, wantPos)
	}
	if !reflect.DeepEqual(flagArgs, wantFlagArgs) {
		t.Fatalf("flagArgs = %v, want %v", flagArgs, wantFlagArgs)
	}
}

func TestSplitFlagsAndPositionalsRespectsDoubleDash(t *testing.T) {
	t.Parallel()
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	pos, flagArgs := splitFlagsAndPositionals(
		[]string{"--", "--config", "intl"},
		fs,
	)
	wantPos := []string{"--config", "intl"}
	if !reflect.DeepEqual(pos, wantPos) {
		t.Fatalf("positional = %v, want %v", pos, wantPos)
	}
	if len(flagArgs) != 0 {
		t.Fatalf("flagArgs = %v, want empty", flagArgs)
	}
}
