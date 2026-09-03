package system

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// useFIFOs points the drain at a throwaway fifo and restores the real paths.
func useFIFOs(t *testing.T, paths ...string) {
	t.Helper()
	real := otaRequestFIFOs
	otaRequestFIFOs = paths
	t.Cleanup(func() { otaRequestFIFOs = real })
}

func makeFIFO(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo %s: %v", path, err)
	}
	return path
}

// openForWrite reports how a writer fares, the way control_center's OTA request
// does. A send on the channel means open+write returned; silence means blocked.
func openForWrite(path string) <-chan error {
	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			done <- err
			return
		}
		defer f.Close()
		_, err = f.WriteString("upgrade_request\n")
		done <- err
	}()
	return done
}

// The bug: with no reader, control_center hangs in open() forever and strands a
// worker. This is the guard that the unblock test below proves something real.
func TestFifoWriteBlocksWithNoReader(t *testing.T) {
	fifo := makeFIFO(t, "upgrade_request")

	done := openForWrite(fifo)
	select {
	case err := <-done:
		t.Fatalf("expected the writer to block with no reader, got %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	// Release the stuck goroutine so the test does not leak it.
	r, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("release open: %v", err)
	}
	defer r.Close()
	if err := <-done; err != nil {
		t.Fatalf("writer failed once a reader appeared: %v", err)
	}
}

func TestOtaDrainUnblocksWriter(t *testing.T) {
	fifo := makeFIFO(t, "upgrade_request")
	useFIFOs(t, fifo)

	d := &otaDrain{log: quietLogger()}
	d.start()
	t.Cleanup(d.stop)

	if got := len(d.held); got != 1 {
		t.Fatalf("expected the drain to hold 1 fifo, holds %d", got)
	}

	select {
	case err := <-openForWrite(fifo):
		if err != nil {
			t.Fatalf("write: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("writer blocked; the drain is not holding a reader")
	}
}

// A writer that arrives after the last one left must still not block, which is
// why the fifo is held O_RDWR rather than reopened per request.
func TestOtaDrainSurvivesWriterChurn(t *testing.T) {
	fifo := makeFIFO(t, "upgrade_request")
	useFIFOs(t, fifo)

	d := &otaDrain{log: quietLogger()}
	d.start()
	t.Cleanup(d.stop)

	for i := range 3 {
		select {
		case err := <-openForWrite(fifo):
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("writer %d blocked after earlier writers closed", i)
		}
	}
}

func TestOtaDrainStopReleases(t *testing.T) {
	fifo := makeFIFO(t, "upgrade_request")
	useFIFOs(t, fifo)

	d := &otaDrain{log: quietLogger()}
	d.start()
	d.stop()

	if d.held != nil {
		t.Fatalf("stop should drop every held fifo, still holds %d", len(d.held))
	}
	// Starting again after a stop has to work, since the cloud toggle flips.
	d.start()
	defer d.stop()
	if got := len(d.held); got != 1 {
		t.Fatalf("restart should re-hold the fifo, holds %d", got)
	}
}

// Paths that are absent or not fifos are skipped rather than created: which
// FIFOs exist varies by model, and a stray regular file would silently swallow
// requests.
func TestOtaDrainSkipsNonFifos(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "not_a_fifo")
	if err := os.WriteFile(regular, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "absent")
	useFIFOs(t, regular, missing)

	d := &otaDrain{log: quietLogger()}
	d.start()
	defer d.stop()

	if len(d.held) != 0 {
		t.Fatalf("expected nothing held, holds %d", len(d.held))
	}
}
