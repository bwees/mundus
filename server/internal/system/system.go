// Package system exposes device-management operations the web UI drives: the
// SwitchBot cloud link, SSH access and authorized keys. mundus runs on the robot
// as wlab (with passwordless sudo from the developer overlay), so these act on
// local files and services directly.
package system

import (
	"os"
	"sync"

	"github.com/bwees/mundus/server/internal/robot"
)

type System struct {
	robot *robot.Robot

	// keyMu serializes the read-modify-write cycles on authorized_keys.
	keyMu sync.Mutex

	// Credentials for the local cloud replacement, set via SetCloudSim.
	simCA, simCert, simKey string
}

func New(r *robot.Robot) *System {
	return &System{robot: r}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
