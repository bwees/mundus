// Package system exposes device-management operations the web UI drives: the
// SwitchBot cloud link, SSH access and authorized keys. mundus runs on the robot
// as wlab (with passwordless sudo from the developer overlay), so these act on
// local files and services directly.
package system

import (
	"log/slog"
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

	// ota keeps a reader on the vendor OTA request FIFOs while the cloud is
	// disabled; without it the robot stops cleaning. See otadrain.go.
	ota otaDrain
}

func New(r *robot.Robot, log *slog.Logger) *System {
	if log == nil {
		log = slog.Default()
	}
	return &System{robot: r, ota: otaDrain{log: log}}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
