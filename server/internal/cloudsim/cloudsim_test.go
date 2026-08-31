package cloudsim

import (
	"crypto/x509"
	"os"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Config{CertDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// The endpoint hostname is per-device, so the certificate is minted from the
// SNI the client asks for rather than from a hardcoded name.
func TestCertForMatchesRequestedHost(t *testing.T) {
	s := newTestServer(t)
	host := "a2alhn2dfztqv9-ats.iot.us-east-1.amazonaws.com"

	cert, err := s.certFor(host)
	if err != nil {
		t.Fatalf("certFor: %v", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := leaf.VerifyHostname(host); err != nil {
		t.Errorf("cert does not cover %q: %v", host, err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(s.pub)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Errorf("cert does not chain to the local CA: %v", err)
	}
}

func TestCertForCaches(t *testing.T) {
	s := newTestServer(t)
	a, err := s.certFor("host.example")
	if err != nil {
		t.Fatalf("certFor: %v", err)
	}
	b, err := s.certFor("host.example")
	if err != nil {
		t.Fatalf("certFor: %v", err)
	}
	if a != b {
		t.Error("expected the minted certificate to be reused")
	}
}

// The CA must survive restarts: it is installed into the device trust store, so
// regenerating it would silently break the cloud connection until reinstalled.
func TestCAIsReusedAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	first, err := New(Config{CertDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	second, err := New(Config{CertDir: dir})
	if err != nil {
		t.Fatalf("New (reopen): %v", err)
	}
	if !first.pub.Equal(second.pub) {
		t.Error("CA was regenerated instead of reused")
	}
}

func TestClientCredentialIsWritten(t *testing.T) {
	s := newTestServer(t)
	for _, path := range []string{s.CACertPath(), s.ClientCertPath(), s.ClientKeyPath()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", path)
		}
	}
	if info, err := os.Stat(s.ClientKeyPath()); err == nil && info.Mode().Perm() != 0o600 {
		t.Errorf("client key mode = %v, want 0600", info.Mode().Perm())
	}
}
