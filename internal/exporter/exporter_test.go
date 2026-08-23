package exporter

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-sceneops/internal/project"
)

func TestProjectExportIncludesTruthAndExcludesRuntimeState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if _, err := project.NewStore().Create(root, "Export Test"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sceneops", "private-token"), []byte("sk-should-not-export"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Project(root)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var names []string
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	joined := strings.Join(names, "\n")
	if !strings.Contains(joined, "sceneops.project.json") || !strings.Contains(joined, "brief.md") {
		t.Fatalf("export omitted project truth: %v", names)
	}
	if strings.Contains(joined, ".sceneops") || strings.Contains(joined, ".git") {
		t.Fatalf("export leaked runtime/private state: %v", names)
	}
}
