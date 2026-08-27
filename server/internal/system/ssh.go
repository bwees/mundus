package system

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	sshDir     = "/home/wlab/.ssh"
	authKeys   = "/home/wlab/.ssh/authorized_keys"
	sshdConfig = "/etc/ssh/sshd_config"
)

type SSHKey struct {
	Type    string `json:"type"`
	Key     string `json:"key"`
	Comment string `json:"comment"`
}

type SSHStatus struct {
	Enabled bool     `json:"enabled"` // sshd bound to 0.0.0.0 (reachable over the network)
	Keys    []SSHKey `json:"keys"`
}

func ValidatePubKey(line string) (SSHKey, error) {
	line = strings.TrimSpace(line)
	if strings.Contains(line, "PRIVATE KEY") {
		return SSHKey{}, fmt.Errorf("that looks like a private key; paste the public key")
	}
	f := strings.Fields(line)
	if len(f) < 2 {
		return SSHKey{}, fmt.Errorf("not a valid public key")
	}
	switch {
	case f[0] == "ssh-ed25519", f[0] == "ssh-rsa", strings.HasPrefix(f[0], "ecdsa-sha2-"), strings.HasPrefix(f[0], "sk-"):
	default:
		return SSHKey{}, fmt.Errorf("unsupported key type %q", f[0])
	}
	blob, err := base64.StdEncoding.DecodeString(f[1])
	if err != nil {
		return SSHKey{}, fmt.Errorf("key body is not valid base64")
	}
	// An OpenSSH blob starts with a length-prefixed algorithm name, which must
	// match the type field -- this is what rejects a well-formed-looking paste
	// whose body belongs to a different key.
	if name, ok := blobAlgorithm(blob); !ok || name != f[0] {
		return SSHKey{}, fmt.Errorf("key body does not match type %q", f[0])
	}
	k := SSHKey{Type: f[0], Key: f[1]}
	if len(f) > 2 {
		k.Comment = strings.Join(f[2:], " ")
	}
	return k, nil
}

func blobAlgorithm(blob []byte) (string, bool) {
	if len(blob) < 4 {
		return "", false
	}
	n := binary.BigEndian.Uint32(blob[:4])
	if n > 64 || len(blob) < int(4+n) {
		return "", false
	}
	return string(blob[4 : 4+n]), true
}

func (s *System) SSHStatus() SSHStatus {
	st := SSHStatus{Enabled: s.sshEnabled()}
	st.Keys = s.listKeys()
	return st
}

func (s *System) sshEnabled() bool {
	data, err := os.ReadFile(sshdConfig)
	if err != nil {
		return false
	}
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "ListenAddress") && strings.Contains(ln, "0.0.0.0") {
			return true
		}
	}
	return false
}

func (s *System) listKeys() []SSHKey {
	data, err := os.ReadFile(authKeys)
	if err != nil {
		return nil
	}
	var out []SSHKey
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if k, err := ValidatePubKey(ln); err == nil {
			out = append(out, k)
		}
	}
	return out
}

func (s *System) AddKey(pub string) error {
	k, err := ValidatePubKey(pub)
	if err != nil {
		return err
	}
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	for _, e := range s.listKeys() {
		if e.Key == k.Key {
			return nil
		}
	}
	lines := s.keyLines()
	return writeAuthKeys(append(lines, strings.TrimSpace(k.Type+" "+k.Key+" "+k.Comment)))
}

func (s *System) RemoveKey(blob string) error {
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	var kept []string
	for _, ln := range s.keyLines() {
		f := strings.Fields(ln)
		if len(f) >= 2 && f[1] == blob {
			continue
		}
		kept = append(kept, ln)
	}
	return writeAuthKeys(kept)
}

func (s *System) keyLines() []string {
	data, err := os.ReadFile(authKeys)
	if err != nil {
		return nil
	}
	var out []string
	for _, ln := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// writeAuthKeys replaces authorized_keys atomically. sshd's StrictModes rejects
// the file unless .ssh is 0700 and authorized_keys 0600, and os.WriteFile will
// not narrow the mode of a file that already exists -- hence the explicit Chmod.
func writeAuthKeys(lines []string) error {
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return err
	}
	var buf string
	if len(lines) > 0 {
		buf = strings.Join(lines, "\n") + "\n"
	}
	tmp := filepath.Join(sshDir, ".authorized_keys.tmp")
	if err := os.WriteFile(tmp, []byte(buf), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, authKeys); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Requires the passwordless sudo the installer grants wlab.
func (s *System) SetSSHEnabled(enabled bool) error {
	addr := "127.0.0.1"
	if enabled {
		addr = "0.0.0.0"
	}
	data, err := os.ReadFile(sshdConfig)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "ListenAddress") {
			lines[i] = "ListenAddress " + addr
			found = true
		}
	}
	if !found {
		lines = append(lines, "ListenAddress "+addr)
	}
	if err := s.sudoWrite(sshdConfig, strings.Join(lines, "\n")); err != nil {
		return err
	}
	return s.run("sudo", "systemctl", "restart", "sshd")
}

func (s *System) sudoWrite(path, content string) error {
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo write %s: %w: %s", path, err, out)
	}
	return nil
}

func (s *System) run(name string, args ...string) error {
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, out)
	}
	return nil
}
