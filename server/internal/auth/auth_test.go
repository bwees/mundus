package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Load(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUnconfiguredStore(t *testing.T) {
	s := newStore(t)
	if s.Configured() {
		t.Error("fresh store reports configured")
	}
	if s.Verify("") || s.Verify("anything") {
		t.Error("unconfigured store verified a password")
	}
}

func TestSetupThenVerify(t *testing.T) {
	s := newStore(t)
	if err := s.Setup("correct-horse"); err != nil {
		t.Fatal(err)
	}
	if !s.Configured() {
		t.Error("not configured after setup")
	}
	if !s.Verify("correct-horse") {
		t.Error("correct password rejected")
	}
	if s.Verify("wrong") {
		t.Error("wrong password accepted")
	}
}

// The setup route is unauthenticated, so it must not be usable to overwrite the
// credential on a device that already has one.
func TestSetupRefusesWhenConfigured(t *testing.T) {
	s := newStore(t)
	if err := s.Setup("first-password"); err != nil {
		t.Fatal(err)
	}
	if err := s.Setup("attacker-password"); err == nil {
		t.Fatal("second setup accepted")
	}
	if !s.Verify("first-password") {
		t.Error("original password no longer works")
	}
}

func TestShortPasswordRejected(t *testing.T) {
	s := newStore(t)
	if err := s.Setup("short"); err == nil {
		t.Error("accepted a password below the minimum length")
	}
	if s.Configured() {
		t.Error("store configured despite rejected password")
	}
}

func TestChangePassword(t *testing.T) {
	s := newStore(t)
	if err := s.Setup("first-password"); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePassword("wrong", "second-password"); err == nil {
		t.Error("changed password without the current one")
	}
	if err := s.ChangePassword("first-password", "second-password"); err != nil {
		t.Fatal(err)
	}
	if s.Verify("first-password") || !s.Verify("second-password") {
		t.Error("password did not change")
	}
}

func TestPersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	first, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Setup("correct-horse"); err != nil {
		t.Fatal(err)
	}
	second, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Configured() || !second.Verify("correct-horse") {
		t.Error("credential did not survive reload")
	}
}

func TestFileIsNotWorldReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Setup("correct-horse"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode %o, want 600", mode)
	}
}

func TestPlaintextNeverHitsDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	const password = "correct-horse-battery"
	if err := s.Setup(password); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(data), password) {
		t.Error("plaintext password found in the stored file")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}
