package exporter

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/fountain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
)

type Result struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Files  int    `json:"files"`
}

func Project(root string) (Result, error) {
	snapshot, err := project.NewStore().Open(root)
	if err != nil {
		return Result{}, err
	}
	generated, err := prepareArtifacts(root, snapshot)
	if err != nil {
		return Result{}, err
	}
	name := slug(snapshot.Project.Name)
	if name == "" {
		name = "dramaops-series"
	}
	relative := filepath.Join("exports", fmt.Sprintf("%s-%s.dramaops.zip", name, time.Now().UTC().Format("20060102T150405Z")))
	destination, err := project.ResolveRelative(root, relative)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return Result{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".dramaops-export-*")
	if err != nil {
		return Result{}, err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = tmp.Close(); _ = os.Remove(tmpName) }
	paths, err := exportPaths(root, generated)
	if err != nil {
		cleanup()
		return Result{}, err
	}
	writer := zip.NewWriter(tmp)
	for _, path := range paths {
		rel, _ := filepath.Rel(root, path)
		info, statErr := os.Lstat(path)
		if statErr != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			_ = writer.Close()
			cleanup()
			return Result{}, fmt.Errorf("refusing to export symlink %s", filepath.ToSlash(rel))
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, err
		}
		header.Name, header.Method = filepath.ToSlash(rel), zip.Deflate
		header.SetModTime(time.Unix(0, 0).UTC())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, err
		}
		file, err := os.Open(path)
		if err != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, err
		}
		_, copyErr := io.Copy(entry, file)
		_ = file.Close()
		if copyErr != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, copyErr
		}
	}
	if err := writer.Close(); err != nil {
		cleanup()
		return Result{}, err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return Result{}, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return Result{}, err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		cleanup()
		return Result{}, err
	}
	hash, err := project.HashFile(destination)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: destination, SHA256: hash, Files: len(paths)}, nil
}

func prepareArtifacts(root string, snapshot domain.Snapshot) ([]string, error) {
	paths := make([]string, 0, len(snapshot.Episodes)*2+2)
	for _, episode := range snapshot.Episodes {
		content := fountain.Format(episode, snapshot.Scenes, snapshot.Characters)
		path, err := project.ResolveRelative(root, filepath.Join("exports", episode.ID+".fountain"))
		if err != nil {
			return nil, err
		}
		if err := project.AtomicWrite(path, []byte(content), 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, path)
		for _, edit := range snapshot.Edits {
			if edit.EpisodeID != episode.ID {
				continue
			}
			srtPath, err := project.ResolveRelative(root, filepath.Join("exports", episode.ID+".srt"))
			if err != nil {
				return nil, err
			}
			if err := project.AtomicWrite(srtPath, []byte(formatSRT(edit.SubtitleCues)), 0o644); err != nil {
				return nil, err
			}
			paths = append(paths, srtPath)
		}
	}
	continuityPath, err := project.ResolveRelative(root, filepath.Join("exports", "continuity-report.json"))
	if err != nil {
		return nil, err
	}
	if err := writeJSON(continuityPath, snapshot.ContinuityIssues); err != nil {
		return nil, err
	}
	paths = append(paths, continuityPath)
	provenance := make(map[string]domain.Provenance, len(snapshot.Assets))
	for _, asset := range snapshot.Assets {
		provenance[asset.ID] = asset.Provenance
	}
	provenancePath, err := project.ResolveRelative(root, filepath.Join("exports", "provenance.json"))
	if err != nil {
		return nil, err
	}
	if err := writeJSON(provenancePath, provenance); err != nil {
		return nil, err
	}
	paths = append(paths, provenancePath)
	return paths, nil
}

func exportPaths(root string, generated []string) ([]string, error) {
	paths := append([]string{}, generated...)
	for _, name := range []string{project.ProjectManifest, "AGENTS.md"} {
		path := filepath.Join(root, name)
		if info, err := os.Lstat(path); err == nil && !info.IsDir() {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("refusing to export symlink %s", name)
			}
			paths = append(paths, path)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	for _, directory := range []string{"episodes", "characters", "locations", "props", "scenes", "shots", "assets", "runs", "renders"} {
		base := filepath.Join(root, directory)
		if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		if err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return errors.New("project contains a symlink and cannot be exported")
			}
			paths = append(paths, path)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	unique := paths[:0]
	seen := map[string]bool{}
	for _, path := range paths {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}
	return unique, nil
}

func formatSRT(cues []domain.SubtitleCue) string {
	ordered := append([]domain.SubtitleCue(nil), cues...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartSeconds < ordered[j].StartSeconds })
	var output strings.Builder
	for i, cue := range ordered {
		fmt.Fprintf(&output, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(cue.StartSeconds), srtTime(cue.StartSeconds+cue.DurationSeconds), strings.TrimSpace(cue.Text))
	}
	return output.String()
}

func srtTime(value float64) string {
	if value < 0 {
		value = 0
	}
	milliseconds := int64(value*1000 + 0.5)
	hours := milliseconds / 3600000
	milliseconds %= 3600000
	minutes := milliseconds / 60000
	milliseconds %= 60000
	seconds := milliseconds / 1000
	milliseconds %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return project.AtomicWrite(path, append(data, '\n'), 0o644)
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			builder.WriteRune(char)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
