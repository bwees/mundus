package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// Two binaries live side by side and binPath is a symlink to the active one, so
// an update never writes over the executable that is currently running. The
// switch is a symlink rename, which is atomic.
//
// A pending marker records the slot to fall back to. The new binary clears it
// from Confirm once the web server is actually serving; if it never gets that
// far, the boot loop in the init script points the symlink back and restarts.
const (
	slotA       = ".a"
	slotB       = ".b"
	pendingName = ".update-pending"
)

func (u *Updater) pendingPath() string { return u.binPath + pendingName }

// activeSlot reports the slot binPath currently resolves to, adopting a plain
// file left by the installer into slot A on first use.
func (u *Updater) activeSlot() (string, error) {
	info, err := os.Lstat(u.binPath)
	switch {
	case os.IsNotExist(err):
		return "", fmt.Errorf("no binary at %s", u.binPath)
	case err != nil:
		return "", err
	case info.Mode()&os.ModeSymlink == 0:
		// First update on a fresh install: move the real file into slot A and
		// replace it with a symlink.
		if err := os.Rename(u.binPath, u.binPath+slotA); err != nil {
			return "", fmt.Errorf("adopt %s into a slot: %w", u.binPath, err)
		}
		if err := swapSymlink(u.binPath, u.binPath+slotA); err != nil {
			return "", err
		}
		return slotA, nil
	}
	target, err := os.Readlink(u.binPath)
	if err != nil {
		return "", err
	}
	if filepath.Base(target) == filepath.Base(u.binPath)+slotB {
		return slotB, nil
	}
	return slotA, nil
}

func other(slot string) string {
	if slot == slotA {
		return slotB
	}
	return slotA
}

// swapSymlink points link at target atomically: a symlink cannot be rewritten
// in place, so it is created under a temporary name and renamed over.
func swapSymlink(link, target string) error {
	tmp := link + ".swap"
	os.Remove(tmp)
	if err := os.Symlink(filepath.Base(target), tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, link); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Confirm marks the running binary good and is called once the web server is
// serving. Until it runs, the init loop treats the update as failed.
func (u *Updater) Confirm() {
	if _, err := os.Stat(u.pendingPath()); err != nil {
		return
	}
	if err := os.Remove(u.pendingPath()); err != nil {
		u.log.Error("could not clear the pending-update marker", "err", err)
		return
	}
	u.log.Info("update confirmed", "version", u.Status().CurrentVersion)
}

// Rollback points the symlink back at the slot named in the pending marker.
// The init script calls this via -rollback when mundus exits without confirming.
func (u *Updater) Rollback() error {
	data, err := os.ReadFile(u.pendingPath())
	if err != nil {
		return err
	}
	slot := string(data)
	if slot != slotA && slot != slotB {
		return fmt.Errorf("pending marker names unknown slot %q", slot)
	}
	if err := swapSymlink(u.binPath, u.binPath+slot); err != nil {
		return err
	}
	os.Remove(u.pendingPath())
	u.log.Warn("rolled back to the previous binary", "slot", slot)
	return nil
}
