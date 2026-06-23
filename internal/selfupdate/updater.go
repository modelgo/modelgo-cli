package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// execLookPath is swapped in tests to provide controlled binary resolution.
// Tests that mutate it must not call t.Parallel().
var execLookPath = exec.LookPath

// InstallMethod describes how the CLI binary was installed.
type InstallMethod int

const (
	// InstallNpm means the binary lives under a node_modules tree (npm -g).
	InstallNpm InstallMethod = iota
	// InstallManual means a manually-placed binary (GitHub release download).
	InstallManual
)

const (
	npmInstallTimeout   = 10 * time.Minute
	skillsUpdateTimeout = 2 * time.Minute
	verifyTimeout       = 10 * time.Second
)

// DetectResult holds installation detection results.
type DetectResult struct {
	Method       InstallMethod
	ResolvedPath string
	NpmAvailable bool
}

// CanAutoUpdate reports whether the CLI can update itself automatically.
func (d DetectResult) CanAutoUpdate() bool {
	return d.Method == InstallNpm && d.NpmAvailable
}

// ManualReason explains why auto-update is unavailable.
func (d DetectResult) ManualReason() string {
	if d.Method == InstallNpm && !d.NpmAvailable {
		return "installed via npm, but npm is not available in PATH"
	}
	return "not installed via npm"
}

// CmdResult holds the output of an npm/npx subprocess.
type CmdResult struct {
	Stdout bytes.Buffer
	Stderr bytes.Buffer
	Err    error
}

// CombinedOutput returns stdout + stderr concatenated.
func (r *CmdResult) CombinedOutput() string {
	return r.Stdout.String() + r.Stderr.String()
}

// Updater manages self-update operations. Override the *Override fields in
// tests. Platform-specific PrepareSelfReplace / CleanupStaleFiles /
// CanRestorePreviousVersion live in updater_unix.go and updater_windows.go.
type Updater struct {
	DetectOverride           func() DetectResult
	NpmInstallOverride       func(version string) *CmdResult
	SkillsCommandOverride    func(args ...string) *CmdResult
	VerifyOverride           func(expectedVersion string) error
	RestoreAvailableOverride func() bool

	// backupCreated is set by PrepareSelfReplace (Windows) when the running
	// binary is renamed to .old. Reported by CanRestorePreviousVersion.
	backupCreated bool
}

// New creates an Updater with default (real) behavior.
func New() *Updater { return &Updater{} }

// DetectInstallMethod determines how the CLI was installed and whether npm is
// available for an auto-update.
func (u *Updater) DetectInstallMethod() DetectResult {
	if u.DetectOverride != nil {
		return u.DetectOverride()
	}
	exe, err := os.Executable()
	if err != nil {
		return DetectResult{Method: InstallManual}
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return DetectResult{Method: InstallManual, ResolvedPath: exe}
	}

	method := InstallManual
	if strings.Contains(resolved, "node_modules") {
		method = InstallNpm
	}

	npmAvailable := false
	if method == InstallNpm {
		if _, err := exec.LookPath("npm"); err == nil {
			npmAvailable = true
		}
	}

	return DetectResult{Method: method, ResolvedPath: resolved, NpmAvailable: npmAvailable}
}

// RunNpmInstall executes `npm install -g @model-go/cli@<version>`.
func (u *Updater) RunNpmInstall(version string) *CmdResult {
	if u.NpmInstallOverride != nil {
		return u.NpmInstallOverride(version)
	}
	r := &CmdResult{}
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		r.Err = fmt.Errorf("npm not found in PATH: %w", err)
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), npmInstallTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, npmPath, "install", "-g", NpmPackage+"@"+version)
	cmd.Stdout = &r.Stdout
	cmd.Stderr = &r.Stderr
	r.Err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		r.Err = fmt.Errorf("npm install timed out after %s", npmInstallTimeout)
	}
	return r
}

// SyncSkills runs `npx -y skills add modelgo/modelgo-cli -y -g` so newly
// added skills land alongside the upgraded binary.
func (u *Updater) SyncSkills() *CmdResult {
	return u.runSkillsCommand("-y", "skills", "add", SkillsRepo, "-y", "-g")
}

func (u *Updater) runSkillsCommand(args ...string) *CmdResult {
	if u.SkillsCommandOverride != nil {
		return u.SkillsCommandOverride(args...)
	}
	r := &CmdResult{}
	npxPath, err := exec.LookPath("npx")
	if err != nil {
		r.Err = fmt.Errorf("npx not found in PATH: %w", err)
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), skillsUpdateTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, npxPath, args...)
	cmd.Stdout = &r.Stdout
	cmd.Stderr = &r.Stderr
	r.Err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		r.Err = fmt.Errorf("skills sync timed out after %s", skillsUpdateTimeout)
	}
	return r
}

// VerifyBinary checks that the installed binary reports the expected version.
// `modelgo --version` prints a single token (e.g. "v0.1.5"); the last field is
// extracted and compared against expectedVersion (both stripped of "v").
func (u *Updater) VerifyBinary(expectedVersion string) error {
	if u.VerifyOverride != nil {
		return u.VerifyOverride(expectedVersion)
	}
	// Prefer PATH resolution so the npm global bin symlink picks up the newly
	// installed binary; fall back to the running executable.
	exe, err := execLookPath("modelgo")
	if err != nil {
		exe, err = os.Executable()
		if err != nil {
			return fmt.Errorf("cannot locate binary: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "--version").Output()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("binary verification timed out after %s", verifyTimeout)
	}
	if err != nil {
		return fmt.Errorf("binary not executable: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return fmt.Errorf("empty version output")
	}
	actual := strings.TrimPrefix(fields[len(fields)-1], "v")
	expected := strings.TrimPrefix(expectedVersion, "v")
	if actual != expected {
		return fmt.Errorf("expected version %s, got %q", expectedVersion, actual)
	}
	return nil
}

// Truncate returns the last maxLen runes of s (for trimming noisy npm output).
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[len(r)-maxLen:])
}

// resolveExe returns the resolved path of the current running binary.
func (u *Updater) resolveExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}
