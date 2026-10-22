package system

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func useArchive(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	real := recorderArchiveDir
	recorderArchiveDir = dir
	t.Cleanup(func() { recorderArchiveDir = real })
	return dir
}

func writeAged(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func archiveNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestPruneKeepsNewestBags(t *testing.T) {
	dir := useArchive(t)
	now := time.Now()
	extra := 3
	for i := range recorderKeepBags + extra {
		writeAged(t, dir, fmt.Sprintf("bag_%02d.db3.zip", i), now.Add(-time.Duration(i)*time.Minute))
	}

	(&recorderPrune{log: quietLogger()}).prune(now)

	got := archiveNames(t, dir)
	if len(got) != recorderKeepBags {
		t.Fatalf("kept %d bags, want %d: %v", len(got), recorderKeepBags, got)
	}
	if got[len(got)-1] != fmt.Sprintf("bag_%02d.db3.zip", recorderKeepBags-1) {
		t.Fatalf("pruned the wrong bags: %v", got)
	}
}

func TestPruneRemovesStaleFilesOfAnyKind(t *testing.T) {
	dir := useArchive(t)
	now := time.Now()
	stale := now.Add(-recorderMaxAge - time.Hour)
	writeAged(t, dir, "old.db3", stale)
	writeAged(t, dir, "old.db3.zip", stale)
	writeAged(t, dir, "fresh.db3", now)
	writeAged(t, dir, "fresh.db3.zip", now)

	(&recorderPrune{log: quietLogger()}).prune(now)

	got := archiveNames(t, dir)
	want := []string{"fresh.db3", "fresh.db3.zip"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("archive = %v, want %v", got, want)
	}
}

func TestPruneToleratesMissingArchive(t *testing.T) {
	dir := useArchive(t)
	recorderArchiveDir = filepath.Join(dir, "missing")

	(&recorderPrune{log: quietLogger()}).prune(time.Now())
}

func TestPruneStartStopIsIdempotent(t *testing.T) {
	useArchive(t)
	p := &recorderPrune{log: quietLogger()}

	p.start()
	p.start()
	p.stop()
	p.stop()
}
