// Package auth holds the device's single admin credential. The password is
// stored only as a bcrypt hash; the web UI authenticates with a bearer token
// signed by the server, so there is no session state to keep on disk.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const MinPasswordLength = 8

type Store struct {
	path string
	mu   sync.RWMutex
	hash []byte
}

type storedFile struct {
	PasswordHash string `json:"password_hash"`
}

func Load(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f storedFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s.hash = []byte(f.PasswordHash)
	return s, nil
}

func (s *Store) Configured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.hash) > 0
}

// Setup writes the first admin password. It refuses once one exists, so the
// unauthenticated setup route cannot take over an already-configured device.
func (s *Store) Setup(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.hash) > 0 {
		return fmt.Errorf("already configured")
	}
	return s.storeLocked(password)
}

func (s *Store) Verify(password string) bool {
	s.mu.RLock()
	hash := s.hash
	s.mu.RUnlock()
	if len(hash) == 0 {
		return false
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
}

func (s *Store) ChangePassword(current, next string) error {
	if !s.Verify(current) {
		return fmt.Errorf("current password is incorrect")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storeLocked(next)
}

func (s *Store) storeLocked(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(storedFile{PasswordHash: string(hash)}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return err
	}
	s.hash = hash
	return nil
}
