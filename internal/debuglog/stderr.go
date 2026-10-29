package debuglog

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// StderrPath returns the file that receives fd 2 while the TUI is running.
func StderrPath() string {
	return filepath.Join(filepath.Dir(LogPath()), "stderr.log")
}

// RedirectStderr points fd 2 at StderrPath until the returned restore func is
// called. Anything written to fd 2 lands on the terminal underneath the TUI
// and corrupts the screen — notably macOS libmalloc's
// "MallocStackLogging: can't turn off malloc stack logging" line, which a
// forked child prints between fork and exec, before exec.Cmd has swapped its
// stderr for /dev/null, so cmd.Stderr can't silence it. Only the parent's fd 2
// decides where it goes. On failure fd 2 is left untouched and restore is a
// no-op. restore is idempotent.
func RedirectStderr() (restore func()) {
	path := StderrPath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	truncateIfNeeded(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		Logger.Warn("stderr redirect: open failed", "err", err)
		return func() {}
	}
	defer f.Close()

	saved, err := unix.Dup(int(os.Stderr.Fd()))
	if err != nil {
		Logger.Warn("stderr redirect: dup failed", "err", err)
		return func() {}
	}
	unix.CloseOnExec(saved)
	if err := unix.Dup2(int(f.Fd()), int(os.Stderr.Fd())); err != nil {
		Logger.Warn("stderr redirect: dup2 failed", "err", err)
		unix.Close(saved)
		return func() {}
	}

	restored := false
	return func() {
		if restored {
			return
		}
		restored = true
		if err := unix.Dup2(saved, int(os.Stderr.Fd())); err != nil {
			Logger.Warn("stderr redirect: restore failed", "err", err)
		}
		unix.Close(saved)
	}
}
