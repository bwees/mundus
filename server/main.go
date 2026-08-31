package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bwees/mundus/server/internal/auth"
	"github.com/bwees/mundus/server/internal/cloudsim"
	"github.com/bwees/mundus/server/internal/config"
	"github.com/bwees/mundus/server/internal/funcapi"
	"github.com/bwees/mundus/server/internal/hass"
	"github.com/bwees/mundus/server/internal/robot"
	"github.com/bwees/mundus/server/internal/robotapi"
	"github.com/bwees/mundus/server/internal/system"
	"github.com/bwees/mundus/server/internal/update"
	"github.com/bwees/mundus/server/internal/webapi"
	"github.com/go-fuego/fuego"
)

// version is set via -ldflags "-X main.version=<tag>" in release builds.
var version = "dev"

const updateRepo = "bwees/mundus"

const logKeep = 5

func setupLog(dir string) *slog.Logger {
	stderr := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if dir == "" {
		return stderr
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		stderr.Warn("log dir; logging to stderr only", "dir", dir, "err", err)
		return stderr
	}
	rotateLogs(dir)
	f, err := os.Create(filepath.Join(dir, "mundus.log"))
	if err != nil {
		stderr.Warn("log file; logging to stderr only", "err", err)
		return stderr
	}
	return slog.New(slog.NewTextHandler(io.MultiWriter(os.Stderr, f), nil))
}

func logName(dir string, i int) string {
	if i == 0 {
		return filepath.Join(dir, "mundus.log")
	}
	return filepath.Join(dir, fmt.Sprintf("mundus.%d.log", i))
}

func rotateLogs(dir string) {
	os.Remove(logName(dir, logKeep-1))
	for i := logKeep - 2; i >= 0; i-- {
		_ = os.Rename(logName(dir, i), logName(dir, i+1))
	}
}

// roomsFromMap returns the room list from the map labels (the same source the
// map editor edits) so the dashboard's room chips match renames immediately;
// it falls back to the robot's room query if labels are unavailable.
func roomsFromMap(mapDir string, r *robot.Robot) func() []robot.Room {
	return func() []robot.Room {
		if l, err := robotapi.ReadLabels(mapDir); err == nil && len(l.Data) > 0 {
			out := make([]robot.Room, len(l.Data))
			for i, rl := range l.Data {
				out[i] = robot.Room{ID: rl.ID, Name: rl.Name}
			}
			return out
		}
		rooms, _ := r.Rooms()
		return rooms
	}
}

func main() {
	configPath := flag.String("config", "", "path to JSON config file")
	openapiOut := flag.Bool("openapi", false, "write the web API OpenAPI spec (openapi.json) and exit")
	rollback := flag.Bool("rollback", false, "undo an unconfirmed update and exit (used by the boot loop)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if *openapiOut {
		webapi.NewServer(webapi.Deps{Log: log, SpecMode: true}).OutputOpenAPISpec()
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	log = setupLog(cfg.LogDir)

	cmds := robot.DefaultCommands()
	client := robot.NewClient(cfg.RobotAddr, cfg.RobotDialTimeout, cfg.RobotReadIdle)
	defer client.Close()
	r := robot.New(client, cmds)

	// Acquire the local funcRequest API (runtime token) so commands drive
	// control_center's native funcIDs. Retry: control_center may still be
	// starting when mundus launches.
	var api *robotapi.API
	for attempt := 1; attempt <= 5; attempt++ {
		local, err := funcapi.Acquire(client, "127.0.0.1")
		if err == nil {
			api = robotapi.New(local)
			log.Info("funcapi ready", "url", local.BaseURL, "auth", local.Auth)
			break
		}
		log.Warn("funcapi acquire failed, retrying", "attempt", attempt, "err", err)
		time.Sleep(3 * time.Second)
	}
	if api == nil {
		log.Error("funcapi unavailable; commands will be rejected until it recovers")
	}

	props := robotapi.NewProperties(cfg.PropertyCache)
	rooms := roomsFromMap(cfg.MapDir, r)
	bridge := hass.New(cfg, r, api, props, rooms, log)

	authStore, err := auth.Load(cfg.AuthPath)
	if err != nil {
		log.Error("auth store", "err", err)
		os.Exit(1)
	}
	if !authStore.Configured() {
		log.Warn("no admin password set; open the web UI to finish setup")
	}

	sys := system.New(r)

	// The local cloud replacement runs whenever mundus does. It only takes
	// effect once the cloud is disabled and the resolver points at it, so
	// starting it unconditionally costs nothing and keeps the toggle instant.
	// Failure is not fatal: it only means the cloud cannot be turned off.
	if sim, err := cloudsim.New(cloudsim.Config{
		MQTTAddr: ":8883",
		CertDir:  cfg.CloudSimDir,
		Log:      log,
	}); err != nil {
		log.Error("cloudsim init", "err", err)
	} else if err := sim.Start(); err != nil {
		log.Error("cloudsim start", "err", err)
	} else {
		defer sim.Close()
		sys.SetCloudSim(sim.CACertPath(), sim.ClientCertPath(), sim.ClientKeyPath())
	}

	updater := update.New(updateRepo, version, cfg.BinPath, cfg.WebStatic, log)

	if *rollback {
		if err := updater.Rollback(); err != nil {
			log.Error("rollback", "err", err)
			os.Exit(1)
		}
		return
	}

	if cfg.WebAddr != "" && api != nil {
		security := fuego.NewSecurity()
		security.ExpiresInterval = 30 * 24 * time.Hour
		web := webapi.NewServer(webapi.Deps{
			Auth: authStore, Security: security,
			API: api, Props: props, Robot: r,
			Rooms:       rooms,
			MQTT:        bridge,
			System:      sys,
			Update:      updater,
			MapDir:      cfg.MapDir,
			TrackPath:   cfg.TrackPath,
			RuntimePath: cfg.RuntimePath,
			DeviceName:  cfg.DeviceName,
			Addr:        cfg.WebAddr,
			StaticPath:  cfg.WebStatic,
			Log:         log,
		})
		go func() {
			log.Info("web server starting", "addr", cfg.WebAddr, "static", cfg.WebStatic)
			if err := web.Run(); err != nil {
				log.Error("web server", "err", err)
			}
		}()
		// Serving is the health check: an update that cannot get this far is
		// rolled back by the boot loop.
		go func() {
			time.Sleep(5 * time.Second)
			updater.Confirm()
		}()
	}

	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Info("shutting down")
		close(stop)
	}()

	log.Info("mundus starting", "robot", cfg.RobotAddr, "broker", cfg.MQTTBroker, "device_id", cfg.DeviceID)
	if err := bridge.Run(stop); err != nil {
		log.Error("bridge", "err", err)
		os.Exit(1)
	}
}
