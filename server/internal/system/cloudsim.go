package system

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Wiring for the local cloud stand-in (see internal/cloudsim). Disabling the
// cloud means pointing the robot at mundus instead of at SwitchBot: the trust
// store gets our CA, the endpoint resolves to loopback, and the real client
// credential is parked in favour of a local one.
const (
	vendorCA = "/etc/wlab/ca.pem"
	// certInfo is written at binding time and names the device's own AWS IoT
	// endpoint, which carries a per-device prefix.
	certInfo = "/data/bind/certs/cert_info"
	hostsCfg = "/etc/hosts"
	// control_center reads resolv.conf directly instead of going through
	// nsswitch, so /etc/hosts alone does not steer it. Pointing it at
	// systemd-resolved's stub does: the stub answers from /etc/hosts, and needs
	// no privileged port of our own.
	resolvConf   = "/etc/resolv.conf"
	resolvedStub = "127.0.0.53"
	backupSuffix = ".mundus-backup"
	hostsMarker  = "# mundus-cloudsim"
)

// CloudEndpoint returns the device's AWS IoT hostname.
func CloudEndpoint() (string, error) { return cloudEndpointFrom(certInfo) }

func cloudEndpointFrom(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read cert_info: %w", err)
	}
	var info struct {
		Data struct {
			MQTT struct {
				Address string `json:"address"`
			} `json:"mqtt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("parse cert_info: %w", err)
	}
	if info.Data.MQTT.Address == "" {
		return "", fmt.Errorf("cert_info has no mqtt address")
	}
	return info.Data.MQTT.Address, nil
}

// InstallCloudSim points the robot's cloud endpoint at the local simulator.
func (s *System) InstallCloudSim(caPath, clientCert, clientKey string) error {
	endpoint, err := CloudEndpoint()
	if err != nil {
		return err
	}
	if err := s.backupOnce(vendorCA); err != nil {
		return err
	}
	ca, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("read local CA: %w", err)
	}
	if err := s.sudoWrite(vendorCA, string(ca)); err != nil {
		return fmt.Errorf("install local CA: %w", err)
	}

	// Park the SwitchBot credential and stand a local one in its place, so a
	// resolver failure cannot become an authenticated session against the real
	// cloud -- the parked key is the only one SwitchBot would accept.
	if err := s.parkVendorCerts(); err != nil {
		return err
	}
	if err := copyFile(clientCert, cloudCert, 0o644); err != nil {
		return err
	}
	if err := copyFile(clientKey, cloudKey, 0o600); err != nil {
		return err
	}

	if err := s.writeHosts(endpoint); err != nil {
		return err
	}
	if err := s.backupOnce(resolvConf); err != nil {
		return err
	}
	if err := s.run("sudo", "rm", "-f", resolvConf); err != nil {
		return err
	}
	if err := s.sudoWrite(resolvConf, "nameserver "+resolvedStub+"\n"); err != nil {
		return err
	}
	if err := s.run("sudo", "resolvectl", "flush-caches"); err != nil {
		return err
	}
	// control_center caches both the trust store and the resolved endpoint for
	// the life of the process, so it has to be restarted to pick either up.
	return s.restartApp()
}

// RemoveCloudSim puts the vendor CA, hosts file and resolver back. Restoring the
// real client credential belongs to SetCloudEnabled, which owns that decision.
func (s *System) RemoveCloudSim() error {
	var errs []string
	if fileExists(vendorCA + backupSuffix) {
		if err := s.run("sudo", "cp", vendorCA+backupSuffix, vendorCA); err != nil {
			errs = append(errs, err.Error())
		} else {
			s.run("sudo", "rm", "-f", vendorCA+backupSuffix)
		}
	}
	if err := s.writeHosts(""); err != nil {
		errs = append(errs, err.Error())
	}
	if fileExists(resolvConf + backupSuffix) {
		s.run("sudo", "rm", "-f", resolvConf)
		if err := s.run("sudo", "cp", "-a", resolvConf+backupSuffix, resolvConf); err != nil {
			errs = append(errs, err.Error())
		} else {
			s.run("sudo", "rm", "-f", resolvConf+backupSuffix)
		}
	}
	s.run("sudo", "resolvectl", "flush-caches")
	if len(errs) > 0 {
		return fmt.Errorf("remove cloudsim: %s", strings.Join(errs, "; "))
	}
	return nil
}

// writeHosts rewrites the marked mundus line in /etc/hosts. An empty endpoint
// removes it. Only the marked line is ever touched.
func (s *System) writeHosts(endpoint string) error {
	data, err := os.ReadFile(hostsCfg)
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}
	var kept []string
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.Contains(ln, hostsMarker) {
			continue
		}
		kept = append(kept, ln)
	}
	out := strings.TrimRight(strings.Join(kept, "\n"), "\n") + "\n"
	if endpoint != "" {
		out += "127.0.0.1 " + endpoint + " " + hostsMarker + "\n"
	}
	return s.sudoWrite(hostsCfg, out)
}

// backupOnce preserves the original before the first overwrite and never
// clobbers an existing backup, so repeated installs stay reversible.
func (s *System) backupOnce(path string) error {
	if fileExists(path+backupSuffix) || !fileExists(path) {
		return nil
	}
	return s.run("sudo", "cp", "-a", path, path+backupSuffix)
}

func (s *System) parkVendorCerts() error {
	for _, base := range []string{cloudCert, cloudKey} {
		if fileExists(base) && !fileExists(base+cloudParked) {
			if err := os.Rename(base, base+cloudParked); err != nil {
				return fmt.Errorf("park %s: %w", base, err)
			}
		}
	}
	return nil
}

func (s *System) restartApp() error {
	return s.run("sudo", "systemctl", "restart", "app.service")
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}
