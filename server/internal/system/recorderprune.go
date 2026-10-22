package system

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// The recorder writes a bag per clean into this archive, and only
// upload-recorder.service ever deletes them. With the cloud disabled the archive
// grows until /data fills, control_center can no longer save its state, and
// starting a clean does nothing. These limits mirror the vendor uploader's.
var recorderArchiveDir = "/data/cache/recorder/archive"

const (
	recorderBagExt     = ".zip"
	recorderKeepBags   = 10
	recorderMaxAge     = 48 * time.Hour
	recorderPruneEvery = 5 * time.Minute
)

type recorderPrune struct {
	log *slog.Logger

	mu   sync.Mutex
	done chan struct{}
}

func (p *recorderPrune) start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done != nil {
		return
	}
	p.done = make(chan struct{})
	go p.loop(p.done)
	p.log.Info("recorder archive pruning active", "dir", recorderArchiveDir)
}

func (p *recorderPrune) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil {
		return
	}
	close(p.done)
	p.done = nil
	p.log.Info("recorder archive pruning released")
}

func (p *recorderPrune) loop(done <-chan struct{}) {
	tick := time.NewTicker(recorderPruneEvery)
	defer tick.Stop()
	for {
		p.prune(time.Now())
		select {
		case <-done:
			return
		case <-tick.C:
		}
	}
}

func (p *recorderPrune) prune(now time.Time) {
	entries, err := os.ReadDir(recorderArchiveDir)
	if err != nil {
		if !os.IsNotExist(err) {
			p.log.Warn("recorder prune: cannot read archive", "err", err)
		}
		return
	}

	type bag struct {
		path    string
		modTime time.Time
	}
	var bags []bag
	var removed []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(recorderArchiveDir, e.Name())
		if now.Sub(info.ModTime()) > recorderMaxAge {
			removed = append(removed, path)
			continue
		}
		if strings.HasSuffix(e.Name(), recorderBagExt) {
			bags = append(bags, bag{path, info.ModTime()})
		}
	}

	sort.Slice(bags, func(i, j int) bool { return bags[i].modTime.After(bags[j].modTime) })
	if len(bags) > recorderKeepBags {
		for _, b := range bags[recorderKeepBags:] {
			removed = append(removed, b.path)
		}
	}

	for _, path := range removed {
		if err := os.Remove(path); err != nil {
			p.log.Warn("recorder prune: remove failed", "path", path, "err", err)
		}
	}
	if len(removed) > 0 {
		p.log.Info("recorder archive pruned", "removed", len(removed))
	}
}
