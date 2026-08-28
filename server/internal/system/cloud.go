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
// control; parking the cert makes the AWS IoT handshake fail while leaving
// local control, navigation, cleaning, Matter and BLE untouched. Reversible.
const (
	cloudCert   = "/data/bind/certs/certificate.pem.crt"
	cloudKey    = "/data/bind/certs/private.key"
	cloudParked = ".mundus-disabled"
)

// cloudServices reach SwitchBot on their own, independently of control_center's
// MQTT link: log/image/recording uploads, the vendor OTA updater and the frp
// remote-access tunnel. Parking the cert does not touch them, so they are turned
// off alongside it.
var cloudServices = []string{
	"debug_log_push.service",
	"frpc.service",
	"update-robotic.service",
	"upload-recorder.service",
	"upload_image.service",
}

// parkedSuffixes are checked when restoring, so a cert parked by an older tool
// (s20ctl) still re-enables cleanly.
var parkedSuffixes = []string{cloudParked, ".s20ctl-disabled"}

type CloudStatus struct {
	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`
	Bound     bool `json:"bound"`
}

func (s *System) CloudStatus() CloudStatus {
	present := fileExists(cloudCert)
	parked := false
	for _, suf := range parkedSuffixes {
		if fileExists(cloudCert + suf) {
			parked = true
		}
	}
	st := CloudStatus{Enabled: present && !parked}
	if out, err := s.robot.Raw("/cc/iot/cloud_conn/is_connect"); err == nil {
		st.Connected = strings.HasPrefix(strings.TrimSpace(out), "true")
	}
	if out, err := s.robot.Raw("/cc/iot/cloud_conn/is_bind"); err == nil {
		st.Bound = strings.HasPrefix(strings.TrimSpace(out), "true")
	}
	return st
}

func (s *System) SetCloudEnabled(enabled bool) error {
	for _, base := range []string{cloudCert, cloudKey} {
		if enabled {
			for _, suf := range parkedSuffixes {
				if fileExists(base+suf) && !fileExists(base) {
					if err := os.Rename(base+suf, base); err != nil {
						return fmt.Errorf("restore %s: %w", base, err)
					}
				}
			}
		} else if fileExists(base) {
			if err := os.Rename(base, base+cloudParked); err != nil {
				return fmt.Errorf("park %s: %w", base, err)
			}
		}
	}
	return s.setCloudServices(enabled)
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
