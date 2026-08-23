package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
)

func TestProjectRoundTripAndIndexRebuild(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my-film")
	store := NewStore()
	snapshot, err := store.Create(root, "My Film")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Project.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d", snapshot.Project.SchemaVersion)
	}
	scenes := []domain.Scene{{ID: "scene-1", Title: "Arrival"}, {ID: "scene-2", Title: "Departure"}}
	shots := []domain.Shot{
		{ID: "shot-1", SceneID: "scene-1", Title: "Wide arrival", Order: 0, DurationSeconds: 4},
		{ID: "shot-2", SceneID: "scene-1", Title: "Close detail", Order: 1, DurationSeconds: 4},
		{ID: "shot-3", SceneID: "scene-2", Title: "Last look", Order: 0, DurationSeconds: 4},
	}
	snapshot, err = store.ApplyStoryboard(root, scenes, shots)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Scenes) != 2 || len(snapshot.Shots) != 3 {
		t.Fatalf("unexpected storyboard size: %d scenes, %d shots", len(snapshot.Scenes), len(snapshot.Shots))
	}
	indexPath, _ := IndexPath(root)
	if err := os.Remove(indexPath); err != nil {
		t.Fatal(err)
	}
	if err := RebuildIndex(root); err != nil {
		t.Fatal(err)
	}
	if count, err := CountIndexed(root, "shots"); err != nil || count != 3 {
		t.Fatalf("rebuilt shot count = %d, err = %v", count, err)
	}
}

func TestResolveRelativeRejectsEscapesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"../outside", "/tmp/outside", "scenes/../../outside", ""} {
		if _, err := ResolveRelative(root, path); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on Windows")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRelative(root, "linked/file.json"); err == nil {
		t.Fatal("expected symlink traversal to be rejected")
	}
}

func TestAtomicWriteReplacesCompleteContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "manifest.json")
	if err := AtomicWrite(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(path, []byte("new-complete-value"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-complete-value" {
		t.Fatalf("content = %q", data)
	}
	entries, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".sceneops-write-*"))
	if len(entries) != 0 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestUnsupportedSchemaFailsClosed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "future")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(domain.Project{SchemaVersion: 99, ID: "future", Name: "Future"})
	if err := os.WriteFile(filepath.Join(root, ProjectManifest), data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore().Open(root)
	if err == nil || !strings.Contains(err.Error(), "unsupported schemaVersion") {
		t.Fatalf("expected unsupported schema error, got %v", err)
	}
}
