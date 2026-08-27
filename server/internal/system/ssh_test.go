package system

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

func sshBlob(algorithm string, body []byte) string {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(algorithm)))
	out = append(out, algorithm...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	return base64.StdEncoding.EncodeToString(out)
}

func ed25519Blob(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return sshBlob("ssh-ed25519", pub)
}

func TestValidatePubKeyAccepts(t *testing.T) {
	blob := ed25519Blob(t)
	k, err := ValidatePubKey("ssh-ed25519 " + blob + " me@host")
	if err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	if k.Type != "ssh-ed25519" || k.Key != blob || k.Comment != "me@host" {
		t.Errorf("parsed %+v", k)
	}
}

func TestValidatePubKeyRejects(t *testing.T) {
	blob := ed25519Blob(t)
	rsa := sshBlob("ssh-rsa", []byte("body"))
	tests := []struct {
		name string
		line string
	}{
		{"private key", "-----BEGIN OPENSSH PRIVATE KEY-----"},
		{"too few fields", "ssh-ed25519"},
		{"unsupported type", "ssh-dss " + blob},
		{"body not base64", "ssh-ed25519 not!base64!"},
		{"type/body mismatch", "ssh-ed25519 " + rsa},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidatePubKey(tt.line); err == nil {
				t.Errorf("accepted %q", tt.line)
			}
		})
	}
}

// A pasted key carrying newlines must not become extra authorized_keys lines or
// smuggle in sshd options such as command=.
func TestValidatePubKeyFlattensNewlines(t *testing.T) {
	blob := ed25519Blob(t)
	k, err := ValidatePubKey("ssh-ed25519 " + blob + " host\ncommand=\"/bin/sh\" ssh-rsa AAAA")
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(k.Type + " " + k.Key + " " + k.Comment)
	if strings.Contains(line, "\n") {
		t.Errorf("newline survived into %q", line)
	}
}
