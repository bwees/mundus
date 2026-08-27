package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func elf(machine uint16) []byte {
	b := make([]byte, 20)
	copy(b, "\x7fELF")
	b[4] = 2
	binary.LittleEndian.PutUint16(b[18:20], machine)
	return b
}

func TestCheckExecutableAcceptsNativeELF(t *testing.T) {
	if elfMachine == 0 {
		t.Skip("no e_machine mapping for this GOARCH")
	}
	if err := checkExecutable(elf(elfMachine)); err != nil {
		t.Fatalf("native ELF rejected: %v", err)
	}
}

// These are the payloads that would otherwise brick the device: install()
// renames over the running binary before anything tries to exec it.
func TestCheckExecutableRejects(t *testing.T) {
	tests := []struct {
		name string
		bin  []byte
	}{
		{"empty", nil},
		{"one byte", []byte("x")},
		{"shell script", []byte("#!/bin/sh\necho pwned\n")},
		{"truncated ELF header", []byte("\x7fELF\x02")},
		{"32-bit ELF", func() []byte { b := elf(183); b[4] = 1; return b }()},
		{"foreign architecture", elf(0x3e + 0x1000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkExecutable(tt.bin); err == nil {
				t.Errorf("accepted %s", tt.name)
			}
		})
	}
}

func TestVerifyFailsClosedWithoutChecksum(t *testing.T) {
	if err := verify([]byte("payload"), ""); err == nil {
		t.Fatal("missing checksum accepted")
	}
}

func TestVerifyChecksum(t *testing.T) {
	// sha256("payload")
	const want = "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"
	if err := verify([]byte("payload"), want); err != nil {
		t.Errorf("correct checksum rejected: %v", err)
	}
	if err := verify([]byte("payload"), strings.Repeat("0", 64)); err == nil {
		t.Error("wrong checksum accepted")
	}
}

// tarGzOf builds a one-file web bundle in the shape install() expects.
func tarGzOf(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A tar entry that climbs out of the destination must land inside it anyway.
func TestExtractTarGzContainsTraversal(t *testing.T) {
	dst := t.TempDir()
	if err := extractTarGz(tarGzOf(t, "../../escaped.txt", "nope"), dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "escaped.txt")); err != nil {
		t.Errorf("entry did not land inside the destination: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(dst)), "escaped.txt")); err == nil {
		t.Error("entry escaped the destination directory")
	}
}
