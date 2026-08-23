package runtime

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVersionAtLeast(t *testing.T) {
	for _, test := range []struct {
		actual, required string
		want             bool
	}{
		{"0.149.0", "0.149.0", true},
		{"0.150.0", "0.149.0", true},
		{"0.148.9", "0.149.0", false},
		{"0.149.0-alpha.4.1", "0.149.0", false},
		{"0.149.0", "0.149.0-alpha.4.1", true},
	} {
		if got := versionAtLeast(test.actual, test.required); got != test.want {
			t.Fatalf("versionAtLeast(%q, %q) = %v", test.actual, test.required, got)
		}
	}
}

func TestInstallVerifiesChecksumAndBinaryVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test archive contains a POSIX helper executable")
	}
	archive := testArchive(t, "codex-aarch64-test", []byte("#!/bin/sh\necho 'codex-cli 0.149.0'\n"))
	digest := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write(archive) }))
	defer server.Close()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	manifest := Manifest{Version: "0.149.0", Artifacts: map[string]Artifact{platform: {URL: server.URL, SHA256: hex.EncodeToString(digest[:]), Archive: "tar.gz"}}}
	destination := filepath.Join(t.TempDir(), "runtime", "codex")
	manager := &Manager{HTTPClient: server.Client()}
	installed, err := manager.install(context.Background(), manifest, destination)
	if err != nil {
		t.Fatal(err)
	}
	if installed != destination {
		t.Fatalf("installed path = %s", installed)
	}
	if info, err := os.Stat(destination); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("installed binary is not executable: %v %v", info, err)
	}
	manifest.Artifacts[platform] = Artifact{URL: server.URL, SHA256: "deadbeef", Archive: "tar.gz"}
	if _, err := manager.install(context.Background(), manifest, filepath.Join(t.TempDir(), "codex")); err == nil {
		t.Fatal("checksum mismatch must fail closed")
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("bad")
	_ = tarWriter.WriteHeader(&tar.Header{Name: "../codex", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg})
	_, _ = tarWriter.Write(content)
	_ = tarWriter.Close()
	_ = gzipWriter.Close()
	path := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractCodexBinary(path); err == nil {
		t.Fatal("archive traversal must be rejected")
	}
}

func testArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
