package exporter

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bg-dao/axon-codex-sceneops/internal/project"
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
	name := slug(snapshot.Project.Name)
	if name == "" {
		name = "sceneops-project"
	}
	relative := filepath.Join("exports", fmt.Sprintf("%s-%s.sceneops.zip", name, time.Now().UTC().Format("20060102T150405Z")))
	destination, err := project.ResolveRelative(root, relative)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return Result{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".sceneops-export-*")
	if err != nil {
		return Result{}, err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	paths, err := exportPaths(root)
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
		header, headerErr := zip.FileInfoHeader(info)
		if headerErr != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, headerErr
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		header.SetModTime(time.Unix(0, 0).UTC())
		entry, createErr := writer.CreateHeader(header)
		if createErr != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, createErr
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			_ = writer.Close()
			cleanup()
			return Result{}, openErr
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

func exportPaths(root string) ([]string, error) {
	var paths []string
	for _, name := range []string{"sceneops.project.json", "brief.md", "AGENTS.md"} {
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
	for _, directory := range []string{"scenes", "shots", "assets", "runs"} {
		base := filepath.Join(root, directory)
		if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
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
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
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
