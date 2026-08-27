package system

import (
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
	return nil
}
