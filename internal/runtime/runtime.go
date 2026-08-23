package runtime

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strconv"
	"strings"
)

//go:embed codex-runtime.json
var manifestBytes []byte

type Artifact struct {
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Archive string `json:"archive"`
}

type Manifest struct {
	Version   string              `json:"version"`
	Artifacts map[string]Artifact `json:"artifacts"`
}

type Status struct {
	Path       string `json:"path,omitempty"`
	Version    string `json:"version,omitempty"`
	Required   string `json:"required"`
	Source     string `json:"source,omitempty"`
	Compatible bool   `json:"compatible"`
	Error      string `json:"error,omitempty"`
}

type Progress struct {
	Phase   string `json:"phase"`
	Percent int    `json:"percent"`
	Message string `json:"message"`
}

type Manager struct {
	HTTPClient *http.Client
	Progress   func(Progress)
}

func LoadManifest() (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode Codex runtime manifest: %w", err)
	}
	if manifest.Version == "" {
		return Manifest{}, errors.New("Codex runtime manifest has no version")
	}
	return manifest, nil
}

func (m *Manager) Ensure(ctx context.Context) (Status, error) {
	manifest, err := LoadManifest()
	if err != nil {
		return Status{}, err
	}
	if path, err := exec.LookPath("codex"); err == nil {
		version, versionErr := detectVersion(ctx, path)
		if versionErr == nil && versionAtLeast(version, manifest.Version) {
			return Status{Path: path, Version: version, Required: manifest.Version, Source: "system", Compatible: true}, nil
		}
	}
	privatePath, err := privateRuntimePath(manifest.Version)
	if err != nil {
		return Status{}, err
	}
	if version, versionErr := detectVersion(ctx, privatePath); versionErr == nil && versionAtLeast(version, manifest.Version) {
		return Status{Path: privatePath, Version: version, Required: manifest.Version, Source: "private", Compatible: true}, nil
	}
	installed, err := m.install(ctx, manifest, privatePath)
	if err != nil {
		return Status{Required: manifest.Version, Source: "private", Compatible: false, Error: err.Error()}, err
	}
	return Status{Path: installed, Version: manifest.Version, Required: manifest.Version, Source: "private", Compatible: true}, nil
}

func (m *Manager) Detect(ctx context.Context) Status {
	manifest, err := LoadManifest()
	if err != nil {
		return Status{Error: err.Error()}
	}
	status := Status{Required: manifest.Version}
	path, err := exec.LookPath("codex")
	if err != nil {
		status.Error = "Codex CLI not found"
		return status
	}
	version, err := detectVersion(ctx, path)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Path = path
	status.Version = version
	status.Source = "system"
	status.Compatible = versionAtLeast(version, manifest.Version)
	if !status.Compatible {
		status.Error = fmt.Sprintf("Codex %s is older than required %s", version, manifest.Version)
	}
	return status
}

func (m *Manager) install(ctx context.Context, manifest Manifest, destination string) (string, error) {
	platform := goruntime.GOOS + "-" + goruntime.GOARCH
	artifact, ok := manifest.Artifacts[platform]
	if !ok {
		return "", fmt.Errorf("no verified Codex runtime artifact for %s", platform)
	}
	if artifact.Archive != "tar.gz" {
		return "", fmt.Errorf("unsupported Codex archive %q", artifact.Archive)
	}
	m.emit("download", 5, "Downloading pinned Codex runtime")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return "", err
	}
	client := m.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download Codex runtime: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("download Codex runtime: %s", response.Status)
	}
	tmpDir, err := os.MkdirTemp("", "dramaops-codex-runtime-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)
	archivePath := filepath.Join(tmpDir, "codex.tar.gz")
	archiveFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(archiveFile, hash), io.LimitReader(response.Body, 512<<20))
	closeErr := archiveFile.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, artifact.SHA256) {
		return "", fmt.Errorf("Codex runtime checksum mismatch: expected %s, got %s", artifact.SHA256, actual)
	}
	m.emit("verify", 65, "Verified pinned Codex runtime")
	binaryBytes, err := extractCodexBinary(archivePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return "", err
	}
	tmpBinary, err := os.CreateTemp(filepath.Dir(destination), ".codex-install-*")
	if err != nil {
		return "", err
	}
	tmpName := tmpBinary.Name()
	defer os.Remove(tmpName)
	if err := tmpBinary.Chmod(0o755); err != nil {
		_ = tmpBinary.Close()
		return "", err
	}
	if _, err := tmpBinary.Write(binaryBytes); err != nil {
		_ = tmpBinary.Close()
		return "", err
	}
	if err := tmpBinary.Sync(); err != nil {
		_ = tmpBinary.Close()
		return "", err
	}
	if err := tmpBinary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return "", err
	}
	version, err := detectVersion(ctx, destination)
	if err != nil || !versionAtLeast(version, manifest.Version) {
		return "", errors.New("installed Codex runtime failed version verification")
	}
	m.emit("complete", 100, "Codex runtime is ready")
	return destination, nil
}

func (m *Manager) emit(phase string, percent int, message string) {
	if m.Progress != nil {
		m.Progress(Progress{Phase: phase, Percent: percent, Message: message})
	}
}

func privateRuntimePath(version string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "DramaOps", "runtime", "codex", version, "codex"), nil
}

func extractCodexBinary(archivePath string) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	tars := tar.NewReader(gzipReader)
	for {
		header, err := tars.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		clean := filepath.Clean(header.Name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("unsafe path in Codex archive: %q", header.Name)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(clean)
		if base != "codex" && !strings.HasPrefix(base, "codex-") {
			continue
		}
		if header.Size <= 0 || header.Size > 512<<20 {
			return nil, errors.New("invalid Codex binary size")
		}
		return io.ReadAll(io.LimitReader(tars, header.Size))
	}
	return nil, errors.New("Codex archive did not contain a binary")
}

var versionPattern = regexp.MustCompile(`([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z.-]+))?`)

func detectVersion(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", errors.New("Codex path is empty")
	}
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read Codex version: %w", err)
	}
	match := versionPattern.FindStringSubmatch(string(output))
	if len(match) < 4 {
		return "", fmt.Errorf("unrecognized Codex version output")
	}
	return match[0], nil
}

func versionAtLeast(actual, required string) bool {
	parse := func(value string) [3]int {
		match := versionPattern.FindStringSubmatch(value)
		var result [3]int
		if len(match) >= 4 {
			for i := 0; i < 3; i++ {
				result[i], _ = strconv.Atoi(match[i+1])
			}
		}
		return result
	}
	a, r := parse(actual), parse(required)
	for i := 0; i < 3; i++ {
		if a[i] != r[i] {
			return a[i] > r[i]
		}
	}
	actualMatch := versionPattern.FindStringSubmatch(actual)
	requiredMatch := versionPattern.FindStringSubmatch(required)
	actualPrerelease := len(actualMatch) >= 5 && actualMatch[4] != ""
	requiredPrerelease := len(requiredMatch) >= 5 && requiredMatch[4] != ""
	if actualPrerelease != requiredPrerelease {
		return !actualPrerelease
	}
	return true
}
