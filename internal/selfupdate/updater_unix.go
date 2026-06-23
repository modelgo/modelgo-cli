//go:build !windows

package selfupdate

// PrepareSelfReplace is a no-op on Unix: a running executable can be replaced
// in place via inode semantics, so npm can overwrite it without EBUSY.
func (u *Updater) PrepareSelfReplace() (restore func(), err error) {
	return func() {}, nil
}

// CleanupStaleFiles is a no-op on Unix (no .old files are created).
func (u *Updater) CleanupStaleFiles() {}

// CanRestorePreviousVersion reports whether a restorable backup exists for the
// current update attempt. Always false on Unix (no backup is taken).
func (u *Updater) CanRestorePreviousVersion() bool {
	if u.RestoreAvailableOverride != nil {
		return u.RestoreAvailableOverride()
	}
	return u.backupCreated
}
