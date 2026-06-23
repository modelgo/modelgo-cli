//go:build windows

package selfupdate

import (
	"fmt"
	"os"
)

// PrepareSelfReplace renames the running .exe to .old so npm's install can
// write the new binary without hitting a sharing violation. Returns a restore
// function that undoes the rename on failure.
func (u *Updater) PrepareSelfReplace() (restore func(), err error) {
	noop := func() {}

	exe, err := u.resolveExe()
	if err != nil {
		return noop, nil // best-effort; don't block update
	}

	oldPath := exe + ".old"

	// Clean up a stale .old from a previous upgrade.
	os.Remove(oldPath)

	// Windows allows renaming a locked, running executable.
	if err := os.Rename(exe, oldPath); err != nil {
		return noop, fmt.Errorf("cannot rename binary for update: %w", err)
	}
	u.backupCreated = true

	// Restore moves .old back. Guard with Stat so we don't delete the only
	// working binary if .old was already recovered elsewhere.
	restore = func() {
		if _, err := os.Stat(oldPath); err != nil {
			u.backupCreated = false
			return
		}
		os.Remove(exe)
		if err := os.Rename(oldPath, exe); err != nil {
			u.backupCreated = false
		}
	}

	return restore, nil
}

// CleanupStaleFiles removes leftover .old files from previous upgrades, or
// restores .old if the original binary is missing (crash mid-update).
func (u *Updater) CleanupStaleFiles() {
	exe, err := u.resolveExe()
	if err != nil {
		return
	}
	oldPath := exe + ".old"

	if _, err := os.Stat(oldPath); err != nil {
		return // no .old file
	}
	if _, err := os.Stat(exe); err != nil {
		os.Rename(oldPath, exe) // original missing — recover
		return
	}
	os.Remove(oldPath) // both exist — .old is stale
}

// CanRestorePreviousVersion reports whether PrepareSelfReplace created a
// restorable backup for the current update attempt.
func (u *Updater) CanRestorePreviousVersion() bool {
	if u.RestoreAvailableOverride != nil {
		return u.RestoreAvailableOverride()
	}
	return u.backupCreated
}
