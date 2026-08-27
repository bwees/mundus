// Package update self-updates the mundus server from GitHub Releases (or an
// uploaded bundle). A release carries three assets: mundus-linux-<arch> (the
// server binary), web.tar.gz (the built web UI), and SHA256SUMS. Apply verifies
// checksums, swaps the binary + web tree in place, and re-execs.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Status struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	State           string    `json:"state"` // idle | checking | applying | error
	Error           string    `json:"error,omitempty"`
	LastChecked     time.Time `json:"last_checked"`
}

type Updater struct {
	repo    string // e.g. "bwees/mundus"
	binPath string
	webDir  string
	log     *slog.Logger
	http    *http.Client

	mu     sync.Mutex
	status Status
}

func New(repo, version, binPath, webDir string, log *slog.Logger) *Updater {
	return &Updater{
		repo: repo, binPath: binPath, webDir: webDir, log: log,
		http:   &http.Client{Timeout: 120 * time.Second},
		status: Status{CurrentVersion: version, State: "idle"},
	}
}

func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

func (u *Updater) setState(f func(*Status)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	f(&u.status)
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (u *Updater) latest(ctx context.Context) (*ghRelease, error) {
	if u.repo == "" {
		return nil, fmt.Errorf("no update repo configured")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.repo)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github releases: %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func (u *Updater) Check(ctx context.Context) (Status, error) {
	u.setState(func(s *Status) { s.State = "checking"; s.Error = "" })
	rel, err := u.latest(ctx)
	if err != nil {
		u.setState(func(s *Status) { s.State = "error"; s.Error = err.Error(); s.LastChecked = time.Now() })
		return u.Status(), err
	}
	u.setState(func(s *Status) {
		s.State = "idle"
		s.LatestVersion = rel.TagName
		s.UpdateAvailable = rel.TagName != "" && rel.TagName != s.CurrentVersion
		s.LastChecked = time.Now()
	})
	return u.Status(), nil
}

func (u *Updater) binAssetName() string { return "mundus-linux-" + runtime.GOARCH }

func (u *Updater) Apply(ctx context.Context) error {
	rel, err := u.latest(ctx)
	if err != nil {
		return err
	}
	u.setState(func(s *Status) { s.State = "applying"; s.Error = "" })

	assets := map[string]string{}
	for _, a := range rel.Assets {
		assets[a.Name] = a.URL
	}
	binURL, webURL, sumURL := assets[u.binAssetName()], assets["web.tar.gz"], assets["SHA256SUMS"]
	if binURL == "" || webURL == "" {
		return u.fail(fmt.Errorf("release %s missing %s or web.tar.gz", rel.TagName, u.binAssetName()))
	}
	if sumURL == "" {
		return u.fail(fmt.Errorf("release %s publishes no SHA256SUMS", rel.TagName))
	}

	b, err := u.download(ctx, sumURL)
	if err != nil {
		return u.fail(err)
	}
	sums := parseSums(string(b))

	bin, err := u.download(ctx, binURL)
	if err != nil {
		return u.fail(err)
	}
	if err := verify(bin, sums[u.binAssetName()]); err != nil {
		return u.fail(fmt.Errorf("binary checksum: %w", err))
	}
	web, err := u.download(ctx, webURL)
	if err != nil {
		return u.fail(err)
	}
	if err := verify(web, sums["web.tar.gz"]); err != nil {
		return u.fail(fmt.Errorf("web checksum: %w", err))
	}

	if err := u.install(bin, web); err != nil {
		return u.fail(err)
	}
	u.setState(func(s *Status) { s.State = "idle"; s.CurrentVersion = rel.TagName; s.UpdateAvailable = false })
	u.restart()
	return nil
}

func (u *Updater) ApplyUpload(bin, webTarGz []byte) error {
	u.setState(func(s *Status) { s.State = "applying"; s.Error = "" })
	if err := u.install(bin, webTarGz); err != nil {
		return u.fail(err)
	}
	u.setState(func(s *Status) { s.State = "idle"; s.UpdateAvailable = false })
	u.restart()
	return nil
}

func (u *Updater) fail(err error) error {
	u.setState(func(s *Status) { s.State = "error"; s.Error = err.Error() })
	u.log.Error("update failed", "err", err)
	return err
}

// elfMachine is the e_machine value of the architecture we are running on.
var elfMachine = map[string]uint16{"arm64": 183, "amd64": 62, "arm": 40}[runtime.GOARCH]

// checkExecutable rejects anything that is not a native ELF for this machine.
// Without it, any short or foreign payload replaces the binary and the next
// exec fails with nothing left to run.
func checkExecutable(bin []byte) error {
	if len(bin) < 20 || string(bin[:4]) != "\x7fELF" {
		return fmt.Errorf("not an ELF executable")
	}
	if bin[4] != 2 {
		return fmt.Errorf("not a 64-bit ELF")
	}
	if got := binary.LittleEndian.Uint16(bin[18:20]); elfMachine != 0 && got != elfMachine {
		return fmt.Errorf("ELF is for machine %d, want %d (%s)", got, elfMachine, runtime.GOARCH)
	}
	return nil
}

// install writes the new binary into the inactive slot and switches to it. The
// running executable is never written over, so a bad payload costs a restart
// rather than the device.
func (u *Updater) install(bin, webTarGz []byte) error {
	if err := checkExecutable(bin); err != nil {
		return fmt.Errorf("rejected update binary: %w", err)
	}
	current, err := u.activeSlot()
	if err != nil {
		return err
	}
	target := u.binPath + other(current)
	if err := os.WriteFile(target, bin, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(target, 0o755); err != nil {
		return err
	}
	// Written before the switch: if power is lost mid-update, the boot loop
	// still knows which slot to go back to.
	if err := os.WriteFile(u.pendingPath(), []byte(current), 0o644); err != nil {
		return err
	}
	if err := swapSymlink(u.binPath, target); err != nil {
		os.Remove(u.pendingPath())
		return err
	}
	if len(webTarGz) > 0 && u.webDir != "" {
		staging := u.webDir + ".new"
		os.RemoveAll(staging)
		if err := os.MkdirAll(staging, 0o755); err != nil {
			return err
		}
		if err := extractTarGz(webTarGz, staging); err != nil {
			return err
		}
		os.RemoveAll(u.webDir + ".old")
		if fileExists(u.webDir) {
			if err := os.Rename(u.webDir, u.webDir+".old"); err != nil {
				return err
			}
		}
		if err := os.Rename(staging, u.webDir); err != nil {
			return err
		}
		os.RemoveAll(u.webDir + ".old")
	}
	return nil
}

// The delay lets the HTTP response return first. syscall.Exec replaces the
// process image in place, which works with or without a supervisor.
func (u *Updater) restart() {
	go func() {
		time.Sleep(750 * time.Millisecond)
		u.log.Info("restarting into updated binary")
		if err := syscall.Exec(u.binPath, os.Args, os.Environ()); err != nil {
			u.log.Error("re-exec failed; exiting so the boot loop rolls back", "err", err)
			os.Exit(1)
		}
	}()
}

func (u *Updater) download(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := u.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func verify(data []byte, want string) error {
	if want == "" {
		return fmt.Errorf("no published checksum")
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
		return fmt.Errorf("mismatch: got %s want %s", got, want)
	}
	return nil
}

func parseSums(s string) map[string]string {
	m := map[string]string{}
	for _, ln := range strings.Split(s, "\n") {
		f := strings.Fields(ln)
		if len(f) == 2 {
			m[strings.TrimPrefix(f[1], "*")] = f[0]
		}
	}
	return m
}

func extractTarGz(data []byte, dst string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.Clean("/"+h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
