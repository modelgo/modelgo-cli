package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectResult_CanAutoUpdate(t *testing.T) {
	cases := []struct {
		d    DetectResult
		want bool
	}{
		{DetectResult{Method: InstallNpm, NpmAvailable: true}, true},
		{DetectResult{Method: InstallNpm, NpmAvailable: false}, false},
		{DetectResult{Method: InstallManual, NpmAvailable: true}, false},
	}
	for _, tc := range cases {
		if got := tc.d.CanAutoUpdate(); got != tc.want {
			t.Errorf("CanAutoUpdate(%+v) = %v, want %v", tc.d, got, tc.want)
		}
	}
}

func TestManualReason(t *testing.T) {
	if got := (DetectResult{Method: InstallNpm, NpmAvailable: false}).ManualReason(); got != "installed via npm, but npm is not available in PATH" {
		t.Errorf("unexpected reason: %q", got)
	}
	if got := (DetectResult{Method: InstallManual}).ManualReason(); got != "not installed via npm" {
		t.Errorf("unexpected reason: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("hello world", 5); got != "world" {
		t.Errorf("Truncate = %q, want %q", got, "world")
	}
	if got := Truncate("hi", 10); got != "hi" {
		t.Errorf("Truncate = %q, want %q", got, "hi")
	}
	if got := Truncate("anything", 0); got != "" {
		t.Errorf("Truncate(_, 0) = %q, want empty", got)
	}
}

// VerifyBinary must accept a binary that prints exactly the expected version
// token (modelgo --version prints just "v0.1.5"), and reject a mismatch.
func TestVerifyBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "modelgo")
	// A tiny shell script standing in for the modelgo binary.
	script := "#!/bin/sh\necho v0.1.6\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	oldLookPath := execLookPath
	execLookPath = func(string) (string, error) { return bin, nil }
	defer func() { execLookPath = oldLookPath }()

	u := New()
	if err := u.VerifyBinary("0.1.6"); err != nil {
		t.Errorf("VerifyBinary(0.1.6) unexpected error: %v", err)
	}
	if err := u.VerifyBinary("v0.1.6"); err != nil {
		t.Errorf("VerifyBinary(v0.1.6) should tolerate v-prefix: %v", err)
	}
	if err := u.VerifyBinary("0.1.5"); err == nil {
		t.Error("VerifyBinary(0.1.5) should fail on mismatch")
	}
}

func TestRunNpmInstall_Override(t *testing.T) {
	u := &Updater{
		NpmInstallOverride: func(version string) *CmdResult {
			if version != "0.1.6" {
				t.Errorf("version = %q, want 0.1.6", version)
			}
			return &CmdResult{}
		},
	}
	if r := u.RunNpmInstall("0.1.6"); r.Err != nil {
		t.Errorf("unexpected err: %v", r.Err)
	}
}

func TestSyncSkills_Override(t *testing.T) {
	var gotArgs []string
	u := &Updater{
		SkillsCommandOverride: func(args ...string) *CmdResult {
			gotArgs = args
			return &CmdResult{}
		},
	}
	u.SyncSkills()
	want := []string{"-y", "skills", "add", SkillsRepo, "-y", "-g"}
	if fmt.Sprint(gotArgs) != fmt.Sprint(want) {
		t.Errorf("SyncSkills args = %v, want %v", gotArgs, want)
	}
}

// Unix builds never create a backup, so rollback is unavailable.
func TestCanRestorePreviousVersion_Default(t *testing.T) {
	u := New()
	if u.CanRestorePreviousVersion() {
		t.Error("fresh updater should report no restorable backup")
	}
}
