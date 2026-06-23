// Package updatecmd implements `modelgo update`: check the npm registry for a
// newer @model-go/cli, and (when installed via npm) self-replace the binary in
// place and re-sync skills. Mirrors the lark-cli update UX.
package updatecmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/modelgo/modelgo-cli/internal/selfupdate"
	"github.com/modelgo/modelgo-cli/internal/version"
)

const maxNpmOutput = 2000

// Overridable for testing.
var (
	fetchLatest    = selfupdate.FetchLatest
	currentVersion = func() string { return version.Version }
	newUpdater     = func() *selfupdate.Updater { return selfupdate.New() }
)

// Run executes `modelgo update`. args is everything after `modelgo update`.
// Exit codes follow the CLI convention: 2 usage error, 1 runtime error, 0 ok.
func Run(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printUsage(stdout)
			return 0
		}
	}

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "write structured JSON output")
	force := fs.Bool("force", false, "reinstall even if already up to date")
	check := fs.Bool("check", false, "only check for updates, do not install")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rest := fs.Args(); len(rest) > 0 {
		fmt.Fprintf(stderr, "update: unexpected argument %q\n", rest[0])
		return 2
	}

	cur := currentVersion()
	updater := newUpdater()
	if !*check {
		updater.CleanupStaleFiles()
	}

	// 1. Fetch latest published version.
	latest, err := fetchLatest()
	if err != nil {
		return reportError(*jsonOut, stdout, stderr, "network",
			fmt.Sprintf("failed to check latest version: %s", err))
	}
	if selfupdate.ParseVersion(latest) == nil {
		return reportError(*jsonOut, stdout, stderr, "update_error",
			fmt.Sprintf("invalid version from registry: %s", latest))
	}

	// 2. Already up to date.
	if !*force && !selfupdate.IsNewer(latest, cur) {
		if !*check {
			syncSkills(updater, *jsonOut, stdout, stderr)
		}
		return reportUpToDate(*jsonOut, stdout, cur, latest)
	}

	detect := updater.DetectInstallMethod()

	// 3. --check: report only, no side effects.
	if *check {
		return reportCheck(*jsonOut, stdout, stderr, cur, latest, detect.CanAutoUpdate())
	}

	// 4. Manual install: cannot self-update.
	if !detect.CanAutoUpdate() {
		return reportManual(*jsonOut, stdout, stderr, cur, latest, detect)
	}

	// 5. npm self-update.
	return doNpmUpdate(*jsonOut, stdout, stderr, cur, latest, updater)
}

func doNpmUpdate(jsonOut bool, stdout, stderr io.Writer, cur, latest string, updater *selfupdate.Updater) int {
	restore, err := updater.PrepareSelfReplace()
	if err != nil {
		return reportError(jsonOut, stdout, stderr, "update_error",
			fmt.Sprintf("failed to prepare update: %s", err))
	}

	if !jsonOut {
		fmt.Fprintf(stderr, "Updating modelgo %s %s %s via npm ...\n", cur, arrow(), latest)
	}

	npmResult := updater.RunNpmInstall(latest)
	if npmResult.Err != nil {
		restore()
		combined := selfupdate.Truncate(npmResult.CombinedOutput(), maxNpmOutput)
		hint := permissionHint(npmResult.CombinedOutput())
		if jsonOut {
			writeJSON(stdout, map[string]any{
				"ok": false,
				"error": map[string]any{
					"type": "update_error", "message": fmt.Sprintf("npm install failed: %s", npmResult.Err),
					"detail": combined, "hint": hint,
				},
			})
			return 1
		}
		if combined != "" {
			fmt.Fprint(stderr, combined)
			if combined[len(combined)-1] != '\n' {
				fmt.Fprintln(stderr)
			}
		}
		fmt.Fprintf(stderr, "%s Update failed: %s\n", cross(), npmResult.Err)
		if hint != "" {
			fmt.Fprintf(stderr, "  %s\n", hint)
		}
		return 1
	}

	// Verify the new binary runs and reports the expected version; roll back on failure.
	if err := updater.VerifyBinary(latest); err != nil {
		restore()
		msg := fmt.Sprintf("new binary verification failed: %s", err)
		hint := verifyHint(updater, latest)
		if jsonOut {
			writeJSON(stdout, map[string]any{
				"ok":    false,
				"error": map[string]any{"type": "update_error", "message": msg, "hint": hint},
			})
			return 1
		}
		fmt.Fprintf(stderr, "%s %s\n  %s\n", cross(), msg, hint)
		return 1
	}

	skills := syncSkills(updater, jsonOut, stdout, stderr)

	if jsonOut {
		writeJSON(stdout, map[string]any{
			"ok": true, "previous_version": cur, "current_version": latest,
			"latest_version": latest, "action": "updated",
			"message": fmt.Sprintf("modelgo updated from %s to %s", cur, latest),
			"url":     selfupdate.ReleaseURL(latest), "changelog": selfupdate.ChangelogURL(),
			"skills_synced": skills,
		})
		return 0
	}
	fmt.Fprintf(stdout, "%s Updated modelgo from %s to %s\n", check(), cur, latest)
	fmt.Fprintf(stdout, "  Changelog: %s\n", selfupdate.ChangelogURL())
	return 0
}

// syncSkills runs the skills re-sync and reports a short status line. Failures
// are non-fatal (the binary update already succeeded). Returns true on success.
func syncSkills(updater *selfupdate.Updater, jsonOut bool, stdout, stderr io.Writer) bool {
	r := updater.SyncSkills()
	if r.Err != nil {
		if !jsonOut {
			fmt.Fprintf(stderr, "%s Skills sync failed: %s\n", warn(), r.Err)
			fmt.Fprintf(stderr, "  Retry: npx skills add %s -y -g\n", selfupdate.SkillsRepo)
		}
		return false
	}
	if !jsonOut {
		fmt.Fprintf(stdout, "%s Skills synced\n", check())
	}
	return true
}

func reportUpToDate(jsonOut bool, stdout io.Writer, cur, latest string) int {
	if jsonOut {
		writeJSON(stdout, map[string]any{
			"ok": true, "previous_version": cur, "current_version": cur,
			"latest_version": latest, "action": "already_up_to_date",
			"message": fmt.Sprintf("modelgo %s is already up to date", cur),
		})
		return 0
	}
	fmt.Fprintf(stdout, "%s modelgo %s is already up to date\n", check(), cur)
	return 0
}

func reportCheck(jsonOut bool, stdout, stderr io.Writer, cur, latest string, canAuto bool) int {
	if jsonOut {
		writeJSON(stdout, map[string]any{
			"ok": true, "current_version": cur, "latest_version": latest,
			"action": "update_available", "auto_update": canAuto,
			"message": fmt.Sprintf("modelgo %s %s %s available", cur, arrow(), latest),
			"url":     selfupdate.ReleaseURL(latest), "changelog": selfupdate.ChangelogURL(),
		})
		return 0
	}
	fmt.Fprintf(stdout, "Update available: %s %s %s\n", cur, arrow(), latest)
	fmt.Fprintf(stdout, "  Release:   %s\n", selfupdate.ReleaseURL(latest))
	fmt.Fprintf(stdout, "  Changelog: %s\n", selfupdate.ChangelogURL())
	if canAuto {
		fmt.Fprintf(stdout, "\nRun `modelgo update` to install.\n")
	} else {
		fmt.Fprintf(stdout, "\nDownload the release above to update manually.\n")
	}
	return 0
}

func reportManual(jsonOut bool, stdout, stderr io.Writer, cur, latest string, detect selfupdate.DetectResult) int {
	reason := detect.ManualReason()
	if jsonOut {
		writeJSON(stdout, map[string]any{
			"ok": true, "previous_version": cur, "latest_version": latest,
			"action":  "manual_required",
			"message": fmt.Sprintf("automatic update unavailable: %s (path: %s)", reason, detect.ResolvedPath),
			"url":     selfupdate.ReleaseURL(latest), "changelog": selfupdate.ChangelogURL(),
		})
		return 0
	}
	fmt.Fprintf(stdout, "Automatic update unavailable: %s (path: %s).\n\n", reason, detect.ResolvedPath)
	fmt.Fprintf(stdout, "To update manually, either download the latest release:\n")
	fmt.Fprintf(stdout, "  Release:   %s\n", selfupdate.ReleaseURL(latest))
	fmt.Fprintf(stdout, "  Changelog: %s\n", selfupdate.ChangelogURL())
	fmt.Fprintf(stdout, "\nor re-run the installer:\n  npx %s@latest install\n", selfupdate.NpmPackage)
	return 0
}

func reportError(jsonOut bool, stdout, stderr io.Writer, errType, msg string) int {
	if jsonOut {
		writeJSON(stdout, map[string]any{
			"ok": false, "error": map[string]any{"type": errType, "message": msg},
		})
		return 1
	}
	fmt.Fprintf(stderr, "update: %s\n", msg)
	return 1
}

func permissionHint(npmOutput string) string {
	if runtime.GOOS != "windows" && strings.Contains(npmOutput, "EACCES") {
		return "Permission denied. Try: sudo modelgo update, or fix your npm global prefix: https://docs.npmjs.com/resolving-eacces-permissions-errors"
	}
	return ""
}

func verifyHint(updater *selfupdate.Updater, latest string) string {
	if updater.CanRestorePreviousVersion() {
		return "the previous version has been restored"
	}
	return fmt.Sprintf("reinstall manually: npx %s@%s install", selfupdate.NpmPackage, latest)
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// --- terminal symbols (ASCII fallback on Windows) ---

func windows() bool { return runtime.GOOS == "windows" }

func check() string {
	if windows() {
		return "[OK]"
	}
	return "✓"
}

func cross() string {
	if windows() {
		return "[FAIL]"
	}
	return "✗"
}

func warn() string {
	if windows() {
		return "[WARN]"
	}
	return "⚠"
}

func arrow() string {
	if windows() {
		return "->"
	}
	return "→"
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `modelgo update — update the modelgo CLI to the latest version

USAGE:
    modelgo update [flags]

Detects how modelgo was installed:
  - npm (global):  runs npm install -g `+selfupdate.NpmPackage+`@<latest>, verifies
                   the new binary (rolls back on failure), then re-syncs skills.
  - manual binary: prints the GitHub release download + installer command.

FLAGS:
    --check    Only check for a newer version; do not install
    --force    Reinstall even if already up to date (also re-syncs skills)
    --json     Write structured JSON output (for agents and scripts)
    --help     Show this help`)
}
