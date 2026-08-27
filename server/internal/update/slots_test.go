package update

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// newTestUpdater lays out a fresh install: binPath is a plain file, exactly as
// the USB installer leaves it.
func newTestUpdater(t *testing.T) *Updater {
	t.Helper()
	if elfMachine == 0 {
		t.Skip("no e_machine mapping for this GOARCH")
	}
	dir := t.TempDir()
	binPath := filepath.Join(dir, "mundus")
	if err := os.WriteFile(binPath, append(elf(elfMachine), []byte("v1")...), 0o755); err != nil {
		t.Fatal(err)
	}
	return New("owner/repo", "v1", binPath, filepath.Join(dir, "web"), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newBinary(tag string) []byte { return append(elf(elfMachine), []byte(tag)...) }

// resolved returns the slot suffix binPath currently points at.
func resolved(t *testing.T, u *Updater) string {
	t.Helper()
	info, err := os.Lstat(u.binPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("binPath is not a symlink")
	}
	target, err := os.Readlink(u.binPath)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Ext(target)
}

func readBin(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data[20:])
}

func TestInstallAdoptsPlainFileAndSwitchesSlot(t *testing.T) {
	u := newTestUpdater(t)
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	if got := resolved(t, u); got != slotB {
		t.Errorf("active slot %q, want %q", got, slotB)
	}
	if got := readBin(t, u.binPath+slotA); got != "v1" {
		t.Errorf("slot A holds %q, want the original v1", got)
	}
	if got := readBin(t, u.binPath+slotB); got != "v2" {
		t.Errorf("slot B holds %q, want v2", got)
	}
	// Following the symlink must reach the new build.
	if got := readBin(t, u.binPath); got != "v2" {
		t.Errorf("binPath resolves to %q, want v2", got)
	}
}

// The whole point of slots: the executable that is running is never written to.
func TestInstallLeavesTheRunningBinaryIntact(t *testing.T) {
	u := newTestUpdater(t)
	original, err := os.ReadFile(u.binPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(u.binPath + slotA)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Error("the previously active slot was modified")
	}
}

func TestInstallWritesPendingMarker(t *testing.T) {
	u := newTestUpdater(t)
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(u.pendingPath())
	if err != nil {
		t.Fatalf("no pending marker: %v", err)
	}
	if string(data) != slotA {
		t.Errorf("marker names %q, want %q", data, slotA)
	}
}

func TestConfirmClearsMarker(t *testing.T) {
	u := newTestUpdater(t)
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	u.Confirm()
	if _, err := os.Stat(u.pendingPath()); !os.IsNotExist(err) {
		t.Error("marker survived Confirm")
	}
	// Confirm on an already-confirmed install is a no-op, not an error.
	u.Confirm()
}

func TestRollbackRestoresPreviousSlot(t *testing.T) {
	u := newTestUpdater(t)
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	if err := u.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := resolved(t, u); got != slotA {
		t.Errorf("active slot %q after rollback, want %q", got, slotA)
	}
	if got := readBin(t, u.binPath); got != "v1" {
		t.Errorf("binPath resolves to %q after rollback, want v1", got)
	}
	if _, err := os.Stat(u.pendingPath()); !os.IsNotExist(err) {
		t.Error("marker survived rollback")
	}
}

// Once an update is confirmed there is nothing to undo; a later crash must not
// silently downgrade the device.
func TestRollbackAfterConfirmIsRefused(t *testing.T) {
	u := newTestUpdater(t)
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	u.Confirm()
	if err := u.Rollback(); err == nil {
		t.Fatal("rollback succeeded with no pending update")
	}
	if got := resolved(t, u); got != slotB {
		t.Errorf("slot changed to %q, want to stay on %q", got, slotB)
	}
}

func TestSuccessiveUpdatesAlternateSlots(t *testing.T) {
	u := newTestUpdater(t)
	for _, step := range []struct {
		tag  string
		want string
	}{{"v2", slotB}, {"v3", slotA}, {"v4", slotB}} {
		if err := u.install(newBinary(step.tag), nil); err != nil {
			t.Fatal(err)
		}
		u.Confirm()
		if got := resolved(t, u); got != step.want {
			t.Errorf("%s landed in %q, want %q", step.tag, got, step.want)
		}
		if got := readBin(t, u.binPath); got != step.tag {
			t.Errorf("binPath resolves to %q, want %s", got, step.tag)
		}
	}
}

// A rejected payload must not switch slots or leave a marker behind, otherwise
// the boot loop would roll back a perfectly good install.
func TestRejectedPayloadLeavesInstallUntouched(t *testing.T) {
	u := newTestUpdater(t)
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	u.Confirm()

	for _, bad := range [][]byte{nil, []byte("#!/bin/sh\necho pwned\n"), elf(0x9999)} {
		if err := u.install(bad, nil); err == nil {
			t.Fatal("bad payload accepted")
		}
		if got := resolved(t, u); got != slotB {
			t.Errorf("slot moved to %q after a rejected payload", got)
		}
		if _, err := os.Stat(u.pendingPath()); !os.IsNotExist(err) {
			t.Error("rejected payload left a pending marker")
		}
		if got := readBin(t, u.binPath); got != "v2" {
			t.Errorf("binPath resolves to %q, want the good v2", got)
		}
	}
}

// Power loss between writing the marker and confirming looks exactly like a
// failed boot: a fresh process must roll back to the recorded slot.
func TestRollbackFromAnotherProcess(t *testing.T) {
	u := newTestUpdater(t)
	if err := u.install(newBinary("v2"), nil); err != nil {
		t.Fatal(err)
	}
	fresh := New("owner/repo", "v2", u.binPath, u.webDir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := fresh.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := readBin(t, u.binPath); got != "v1" {
		t.Errorf("binPath resolves to %q, want v1", got)
	}
}

func TestWebTreeIsReplacedAlongsideTheBinary(t *testing.T) {
	u := newTestUpdater(t)
	if err := os.MkdirAll(u.webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(u.webDir, "index.html"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := u.install(newBinary("v2"), tarGzOf(t, "index.html", "new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(u.webDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("web tree holds %q, want the updated %q", got, "new")
	}
}
