package system

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Cloud enable/disable severs the mutual-TLS client credential the SwitchBot
// cloud MQTT link authenticates with. The cloud client is fused into
// control_center, so it can't be stopped as a service without losing local
// control; parking the cert makes the AWS IoT handshake fail.
//
// Parking the cert alone is not enough, though: the firmware refuses to clean
// without a live cloud session, because BackupMapBag -- the first action of
// every clean task -- is gated on it and otherwise times out after 10s, failing
// the task. So disabling the cloud also stands up the local replacement in
// internal/cloudsim, which satisfies that check without any traffic leaving the
// device. Reversible.
const (
	cloudCert   = "/data/bind/certs/certificate.pem.crt"
	cloudKey    = "/data/bind/certs/private.key"
	cloudParked = ".mundus-disabled"
)

// cloudServices reach SwitchBot on their own, independently of control_center's
// MQTT link: log/image/recording uploads, the vendor OTA updater and the frp
// remote-access tunnel. Parking the cert does not touch them, so they are turned
// off alongside it.
//
// Stopping update-robotic.service has a catch: it is the only reader of the OTA
// request FIFOs, and control_center blocks forever writing to an unread FIFO.
// otadrain.go takes those over whenever this list is disabled.
var cloudServices = []string{
	"debug_log_push.service",
	"frpc.service",
	"update-robotic.service",
	"upload-recorder.service",
	"upload_image.service",
}

type CloudStatus struct {
	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`
	Bound     bool `json:"bound"`
}

// cloudEnabled reads the toggle from disk alone. The file state is the
// authority, and answering without a robot round-trip lets startup consult it
// before the control_center terminal is necessarily up.
func cloudEnabled() bool {
	return fileExists(cloudCert) && !fileExists(cloudCert+cloudParked)
}

func (s *System) CloudStatus() CloudStatus {
	st := CloudStatus{Enabled: cloudEnabled()}
	if out, err := s.robot.Raw("/cc/iot/cloud_conn/is_connect"); err == nil {
		st.Connected = strings.HasPrefix(strings.TrimSpace(out), "true")
	}
	if out, err := s.robot.Raw("/cc/iot/cloud_conn/is_bind"); err == nil {
		st.Bound = strings.HasPrefix(strings.TrimSpace(out), "true")
	}
	return st
}

// SetCloudSim records where the local cloud replacement keeps its credentials.
// Without it, disabling the cloud would break cleaning.
func (s *System) SetCloudSim(caPath, clientCert, clientKey string) {
	s.simCA, s.simCert, s.simKey = caPath, clientCert, clientKey
}

func (s *System) SetCloudEnabled(enabled bool) error {
	if !enabled {
		if s.simCA == "" {
			return fmt.Errorf("local cloud unavailable; disabling would stop the robot cleaning")
		}
		// InstallCloudSim parks the vendor credential itself, so that the real
		// key is never the one presented to the local endpoint.
		if err := s.InstallCloudSim(s.simCA, s.simCert, s.simKey); err != nil {
			return err
		}
		if err := s.setCloudServices(false); err != nil {
			return err
		}
		// update-robotic just went away; take over the OTA request FIFOs before
		// control_center's next request blocks on them for good.
		s.ota.start()
		return nil
	}

	// Hand the FIFOs back before update-robotic is allowed to read them again.
	s.ota.stop()
	if err := s.RemoveCloudSim(); err != nil {
		return err
	}
	for _, base := range []string{cloudCert, cloudKey} {
		// The local stand-in occupies the vendor path while the sim is
		// installed; drop it before restoring the real credential.
		if fileExists(base) && fileExists(base+cloudParked) {
			os.Remove(base)
		}
		if fileExists(base+cloudParked) && !fileExists(base) {
			if err := os.Rename(base+cloudParked, base); err != nil {
				return fmt.Errorf("restore %s: %w", base, err)
			}
		}
	}
	if err := s.setCloudServices(true); err != nil {
		return err
	}
	return s.restartApp()
}

// setCloudServices toggles the vendor units with enable/disable and never with
// mask. Their unit files live in /etc/systemd/system, which is an overlayfs
// lower layer on this device: masking writes a shadow file into the upper, and
// unmasking would delete it through the merged mount, leaving a whiteout that
// hides the vendor unit for good. enable/disable only adds or removes the
// multi-user.target.wants symlink, which the overlay round-trips cleanly.
//
// Re-enabled units are left for the next boot rather than started here:
// update-robotic is a boot-time oneshot and frpc expects a fresh cloud session.
func (s *System) setCloudServices(enabled bool) error {
	var errs []error
	for _, unit := range cloudServices {
		args := []string{"systemctl", "disable", "--now", unit}
		if enabled {
			args = []string{"systemctl", "enable", unit}
		}
		if err := s.run("sudo", args...); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
