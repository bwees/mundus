// Package cloudsim stands in for the SwitchBot cloud on the device itself.
//
// The vendor firmware will not clean without a live cloud session: the first
// action of every clean task, BackupMapBag, is gated on the cloud connection
// and otherwise times out after 10s, failing the whole task. Parking the client
// certificate therefore breaks cleaning outright.
//
// What unblocks it is only that control_center believes it is connected -- the
// S3 credential exchange behind the map-bag upload is never needed. So this
// package answers the AWS IoT endpoint locally: a minimal MQTT broker accepts
// the session and discards everything published to it, while the system wiring
// in internal/system resolves the endpoint to loopback. No robot data reaches
// SwitchBot.
package cloudsim

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config is the runtime wiring for the simulated cloud.
type Config struct {
	// MQTTAddr is where the fake AWS IoT endpoint listens (":8883").
	MQTTAddr string
	// CertDir holds the generated CA and its key, reused across restarts so
	// the CA installed into the system trust store stays valid.
	CertDir string
	Log     *slog.Logger
}

type Server struct {
	cfg Config
	ca  tls.Certificate
	pub *x509.Certificate

	mu      sync.Mutex
	minted  map[string]*tls.Certificate
	session *session
	seq     int
	props   *properties

	listeners []interface{ Close() error }
}

// New loads or creates the local CA. The CA certificate must be installed into
// the device trust store (see CACertPath) before control_center will accept the
// endpoint we present.
func New(cfg Config) (*Server, error) {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	s := &Server{cfg: cfg, minted: map[string]*tls.Certificate{}}
	if err := s.loadOrCreateCA(); err != nil {
		return nil, err
	}
	props, err := loadProperties(cfg.CertDir)
	if err != nil {
		return nil, err
	}
	s.props = props
	return s, nil
}

func (s *Server) CACertPath() string { return filepath.Join(s.cfg.CertDir, "ca.pem") }
func (s *Server) ClientCertPath() string {
	return filepath.Join(s.cfg.CertDir, "client.pem")
}
func (s *Server) ClientKeyPath() string { return filepath.Join(s.cfg.CertDir, "client.key") }

func (s *Server) loadOrCreateCA() error {
	if err := os.MkdirAll(s.cfg.CertDir, 0o755); err != nil {
		return err
	}
	certPath, keyPath := s.CACertPath(), filepath.Join(s.cfg.CertDir, "ca.key")
	if pair, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		leaf, err := x509.ParseCertificate(pair.Certificate[0])
		if err == nil && time.Now().Before(leaf.NotAfter) {
			s.ca, s.pub = pair, leaf
			return s.ensureClientCert()
		}
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "mundus local cloud"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(20, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err := writePEM(certPath, "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	if err := writePEM(keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), 0o600); err != nil {
		return err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	s.ca = tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
	s.pub = leaf
	return s.ensureClientCert()
}

// ensureClientCert writes the stand-in client credential control_center presents
// to us. The real SwitchBot key stays parked, so even a DNS failure cannot turn
// into an authenticated session against the real cloud.
func (s *Server) ensureClientCert() error {
	if _, err := os.Stat(s.ClientCertPath()); err == nil {
		if _, err := os.Stat(s.ClientKeyPath()); err == nil {
			return nil
		}
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "mundus-local-device"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(20, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, s.pub, &key.PublicKey, s.ca.PrivateKey)
	if err != nil {
		return err
	}
	if err := writePEM(s.ClientCertPath(), "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	return writePEM(s.ClientKeyPath(), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), 0o600)
}

// certFor mints (and caches) a server certificate for whatever hostname the
// client asked for. The AWS IoT endpoint is per-device, so deriving it from SNI
// avoids having to know it ahead of time.
func (s *Server) certFor(host string) (*tls.Certificate, error) {
	if host == "" {
		host = "localhost"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.minted[host]; ok {
		return c, nil
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, s.pub, &key.PublicKey, s.ca.PrivateKey)
	if err != nil {
		return nil, err
	}
	c := &tls.Certificate{Certificate: [][]byte{der, s.ca.Certificate[0]}, PrivateKey: key}
	s.minted[host] = c
	return c, nil
}

// Start brings up the MQTT endpoint that stands in for AWS IoT.
func (s *Server) Start() error {
	// The device's TLS stack is old, so the RSA key-exchange suites stay on the
	// menu alongside the modern ones.
	ln, err := tls.Listen("tcp", s.cfg.MQTTAddr, &tls.Config{
		GetCertificate: func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return s.certFor(h.ServerName)
		},
		ClientAuth: tls.RequestClientCert,
		MinVersion: tls.VersionTLS10,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	})
	if err != nil {
		return fmt.Errorf("mqtt listen: %w", err)
	}
	s.listeners = append(s.listeners, ln)
	go s.acceptMQTT(ln)
	s.cfg.Log.Info("cloudsim mqtt listening", "addr", s.cfg.MQTTAddr)
	return nil
}

func (s *Server) Close() error {
	for _, l := range s.listeners {
		l.Close()
	}
	s.listeners = nil
	return nil
}

func writePEM(path, typ string, der []byte, mode os.FileMode) error {
	b := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
	if err := os.WriteFile(path, b, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
