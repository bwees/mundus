package system

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Disabling the cloud stops update-robotic.service, and that service is the only
// reader of the vendor's OTA request FIFOs. control_center goes on writing an OTA
// request periodically, and POSIX blocks open(O_WRONLY) on a FIFO until a reader
// appears -- so with the reader gone every request blocks forever, and each one
// strands a worker from the thread pool control_center shares across background
// work. Once that pool is used up, cc::App::storeBackupMap can no longer get a
// worker, so BackupMapBag -- the first action of every clean task -- never
// completes and dies on its 10s deadline. The robot stops cleaning entirely
// roughly a day after the cloud is switched off, reporting no error code.
//
// Holding the FIFOs open O_RDWR keeps a reader present for as long as mundus
// runs, so those writes complete instead of hanging. Requests are read and
// thrown away, which is exactly what disabling the cloud asked for: the vendor
// OTA updater must not run.
var otaRequestFIFOs = []string{
	"/data/upgrade/upgrade_request",
	"/data/upgrade/bt_upgrade_request",
}

type otaDrain struct {
	log *slog.Logger

	mu   sync.Mutex
	held []*os.File
}

// start takes over every OTA request FIFO that exists. Missing paths are
// skipped: which FIFOs the firmware creates varies by model.
func (d *otaDrain) start() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.held) > 0 {
		return
	}

	for _, path := range otaRequestFIFOs {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeNamedPipe == 0 {
			d.log.Warn("ota drain: not a fifo, leaving alone", "path", path)
			continue
		}
		// O_RDWR rather than O_RDONLY on purpose. A read-only open would itself
		// block until some writer turned up, and would then hit EOF -- ending
		// the drain -- every time the last writer closed, reopening a window
		// for control_center to block in.
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			d.log.Error("ota drain: cannot hold fifo", "path", path, "err", err)
			continue
		}
		d.held = append(d.held, f)
		go d.discard(f)
	}

	if len(d.held) > 0 {
		d.log.Info("ota request fifo drain active", "fifos", len(d.held))
	}
}

func (d *otaDrain) discard(f *os.File) {
	_, err := io.Copy(io.Discard, f)
	if err != nil && !errors.Is(err, os.ErrClosed) {
		d.log.Debug("ota drain: reader stopped", "path", f.Name(), "err", err)
	}
}

// stop hands the FIFOs back, so update-robotic can own them again.
func (d *otaDrain) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.held) == 0 {
		return
	}
	for _, f := range d.held {
		f.Close()
	}
	d.log.Info("ota request fifo drain released", "fifos", len(d.held))
	d.held = nil
}

// SyncOtaDrain matches the drain to the cloud setting. mundus restarts
// independently of the cloud toggle, so this has to run at startup too --
// otherwise a reboot with the cloud already off leaves the FIFOs unread.
func (s *System) SyncOtaDrain() {
	if cloudEnabled() {
		s.ota.stop()
		return
	}
	s.ota.start()
}
