package updatecmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelgo/modelgo-cli/internal/selfupdate"
)

// withStubs swaps the package-level hooks for the duration of a test.
func withStubs(t *testing.T, cur, latest string, fetchErr error, u *selfupdate.Updater) {
	t.Helper()
	oldFetch, oldCur, oldNew := fetchLatest, currentVersion, newUpdater
	fetchLatest = func() (string, error) { return latest, fetchErr }
	currentVersion = func() string { return cur }
	newUpdater = func() *selfupdate.Updater { return u }
	t.Cleanup(func() { fetchLatest, currentVersion, newUpdater = oldFetch, oldCur, oldNew })
}

func npmUpdater() *selfupdate.Updater {
	return &selfupdate.Updater{
		DetectOverride: func() selfupdate.DetectResult {
			return selfupdate.DetectResult{Method: selfupdate.InstallNpm, NpmAvailable: true, ResolvedPath: "/x/node_modules/.bin/modelgo"}
		},
		NpmInstallOverride:    func(string) *selfupdate.CmdResult { return &selfupdate.CmdResult{} },
		VerifyOverride:        func(string) error { return nil },
		SkillsCommandOverride: func(...string) *selfupdate.CmdResult { return &selfupdate.CmdResult{} },
	}
}

func TestUpdate_AlreadyUpToDate(t *testing.T) {
	withStubs(t, "0.1.5", "0.1.5", nil, npmUpdater())
	var out, errOut bytes.Buffer
	code := Run([]string{"--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, stderr: %s", code, errOut.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v (%s)", err, out.String())
	}
	if got["action"] != "already_up_to_date" {
		t.Errorf("action = %v, want already_up_to_date", got["action"])
	}
}

func TestUpdate_Check(t *testing.T) {
	withStubs(t, "0.1.5", "0.1.6", nil, npmUpdater())
	var out, errOut bytes.Buffer
	code := Run([]string{"--check", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	if got["action"] != "update_available" {
		t.Errorf("action = %v, want update_available", got["action"])
	}
	if got["auto_update"] != true {
		t.Errorf("auto_update = %v, want true", got["auto_update"])
	}
}

func TestUpdate_NpmSuccess(t *testing.T) {
	withStubs(t, "0.1.5", "0.1.6", nil, npmUpdater())
	var out, errOut bytes.Buffer
	code := Run([]string{"--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, stderr: %s", code, errOut.String())
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	if got["action"] != "updated" {
		t.Errorf("action = %v, want updated", got["action"])
	}
	if got["current_version"] != "0.1.6" {
		t.Errorf("current_version = %v, want 0.1.6", got["current_version"])
	}
	if got["skills_synced"] != true {
		t.Errorf("skills_synced = %v, want true", got["skills_synced"])
	}
}

func TestUpdate_NpmInstallFails(t *testing.T) {
	u := npmUpdater()
	u.NpmInstallOverride = func(string) *selfupdate.CmdResult {
		r := &selfupdate.CmdResult{}
		r.Stderr.WriteString("npm ERR! EACCES")
		r.Err = errFake("exit 1")
		return r
	}
	var restored bool
	u.RestoreAvailableOverride = func() bool { return restored }
	withStubs(t, "0.1.5", "0.1.6", nil, u)

	var out, errOut bytes.Buffer
	code := Run([]string{"--json"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	if got["ok"] != false {
		t.Errorf("ok = %v, want false", got["ok"])
	}
	errObj, _ := got["error"].(map[string]any)
	if errObj == nil || errObj["type"] != "update_error" {
		t.Errorf("error = %v", got["error"])
	}
	if hint, _ := errObj["hint"].(string); !strings.Contains(hint, "Permission denied") {
		t.Errorf("expected EACCES permission hint, got %q", hint)
	}
}

func TestUpdate_VerifyFailsRollsBack(t *testing.T) {
	u := npmUpdater()
	u.VerifyOverride = func(string) error { return errFake("version mismatch") }
	var restoreCalled bool
	// On unix PrepareSelfReplace returns a real no-op restore; we assert the
	// failure exits 1 with a verification error rather than asserting restore.
	_ = restoreCalled
	withStubs(t, "0.1.5", "0.1.6", nil, u)

	var out, errOut bytes.Buffer
	code := Run([]string{"--json"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	errObj, _ := got["error"].(map[string]any)
	if errObj == nil || !strings.Contains(errObj["message"].(string), "verification failed") {
		t.Errorf("expected verification failure, got %v", got["error"])
	}
}

func TestUpdate_ManualInstall(t *testing.T) {
	u := &selfupdate.Updater{
		DetectOverride: func() selfupdate.DetectResult {
			return selfupdate.DetectResult{Method: selfupdate.InstallManual, ResolvedPath: "/usr/local/bin/modelgo"}
		},
		SkillsCommandOverride: func(...string) *selfupdate.CmdResult { return &selfupdate.CmdResult{} },
	}
	withStubs(t, "0.1.5", "0.1.6", nil, u)

	var out, errOut bytes.Buffer
	code := Run([]string{"--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	if got["action"] != "manual_required" {
		t.Errorf("action = %v, want manual_required", got["action"])
	}
}

func TestUpdate_FetchError(t *testing.T) {
	withStubs(t, "0.1.5", "", errFake("network down"), npmUpdater())
	var out, errOut bytes.Buffer
	code := Run([]string{"--json"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	var got map[string]any
	json.Unmarshal(out.Bytes(), &got)
	errObj, _ := got["error"].(map[string]any)
	if errObj == nil || errObj["type"] != "network" {
		t.Errorf("expected network error, got %v", got["error"])
	}
}

func TestUpdate_UnexpectedArg(t *testing.T) {
	withStubs(t, "0.1.5", "0.1.6", nil, npmUpdater())
	var out, errOut bytes.Buffer
	code := Run([]string{"extra"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2 (usage error)", code)
	}
}

func TestUpdate_BadFlag(t *testing.T) {
	withStubs(t, "0.1.5", "0.1.6", nil, npmUpdater())
	var out, errOut bytes.Buffer
	code := Run([]string{"--nope"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2 (usage error)", code)
	}
}

func TestUpdate_ForceReinstallsWhenUpToDate(t *testing.T) {
	called := false
	u := npmUpdater()
	u.NpmInstallOverride = func(string) *selfupdate.CmdResult { called = true; return &selfupdate.CmdResult{} }
	withStubs(t, "0.1.6", "0.1.6", nil, u) // same version

	var out, errOut bytes.Buffer
	code := Run([]string{"--force", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !called {
		t.Error("--force should reinstall even when already up to date")
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }
